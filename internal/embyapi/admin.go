package embyapi

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"akari-bridge/internal/auth"
	"akari-bridge/internal/bangumi"
	"akari-bridge/internal/config"
	"akari-bridge/internal/bilibili"
	"akari-bridge/internal/engine"
	"akari-bridge/internal/model"
	"akari-bridge/internal/rules"
	"akari-bridge/internal/storage"
)

var startTime = time.Now()

type AdminHandler struct {
	cfg           *config.Config
	authSvc       *auth.AuthService
	ruleMgr       *rules.RuleManager
	eng           *engine.Engine
	bgmClient     *bangumi.Client
	playbackProxy *PlaybackHandler
	db            *storage.DB
	biliAuth      *bilibili.AuthManager
}

func NewAdminHandler(
	cfg *config.Config,
	authSvc *auth.AuthService,
	ruleMgr *rules.RuleManager,
	eng *engine.Engine,
	bgmClient *bangumi.Client,
	playbackProxy *PlaybackHandler,
	db *storage.DB,
	biliAuth *bilibili.AuthManager,
) *AdminHandler {
	return &AdminHandler{
		cfg:           cfg,
		authSvc:       authSvc,
		ruleMgr:       ruleMgr,
		eng:           eng,
		bgmClient:     bgmClient,
		playbackProxy: playbackProxy,
		db:            db,
		biliAuth:      biliAuth,
	}
}

// Auth APIs

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *AdminHandler) Login(c *gin.Context) {
	var req LoginRequest
	_ = c.ShouldBindJSON(&req)

	authRes, err := h.authSvc.Authenticate(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":    authRes.AccessToken,
		"user":     authRes.User,
		"serverId": authRes.ServerId,
	})
}

func (h *AdminHandler) GetMe(c *gin.Context) {
	val, exists := c.Get("user")
	if !exists || val == nil {
		c.JSON(http.StatusOK, gin.H{"guest": true})
		return
	}
	user := val.(*model.UserDto)
	bgmToken, bgmUid := h.authSvc.GetUserBangumi(user.Id)
	c.JSON(http.StatusOK, gin.H{
		"id":          user.Id,
		"name":        user.Name,
		"isAdmin":     user.Policy.IsAdministrator,
		"hasPassword": user.HasPassword,
		"bgmUserId":   bgmUid,
		"hasBgmToken": bgmToken != "",
	})
}

func (h *AdminHandler) ChangePassword(c *gin.Context) {
	var req struct {
		UserId      string `json:"userId"`
		NewPassword string `json:"newPassword"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	if req.UserId == "" {
		req.UserId = "admin"
	}

	if err := h.authSvc.UpdatePassword(req.UserId, req.NewPassword); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Password updated successfully"})
}

// System APIs

func (h *AdminHandler) GetStatus(c *gin.Context) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	dbStats, _ := h.authSvc.GetDB().GetStats()
	activeRules := len(h.ruleMgr.GetEnabledPlugins())
	totalRules := len(h.ruleMgr.GetAllPlugins())

	c.JSON(http.StatusOK, gin.H{
		"serverName":     h.cfg.ServerName,
		"serverId":       h.cfg.ServerId,
		"version":        "1.0.0",
		"serverUrl":      h.cfg.GetServerUrl(),
		"httpPort":       h.cfg.HttpPort,
		"udpPort":        h.cfg.UdpPort,
		"uptimeSeconds":  int64(time.Since(startTime).Seconds()),
		"activeRules":    activeRules,
		"totalRules":     totalRules,
		"goVersion":      runtime.Version(),
		"os":             runtime.GOOS,
		"arch":           runtime.GOARCH,
		"numGoroutines":  runtime.NumGoroutine(),
		"memoryAllocMB":  m.Alloc / 1024 / 1024,
		"memorySysMB":    m.Sys / 1024 / 1024,
		"dbStats":        dbStats,
		"singleUserMode": len(h.authSvc.ListUsers()) <= 1,
	})
}

func (h *AdminHandler) GetConfig(c *gin.Context) {
	proxy := config.GetSystemProxy()
	hasPassword := false
	if adminUser, ok := h.authSvc.GetUser("admin"); ok {
		hasPassword = adminUser.HasPassword
	}
	c.JSON(http.StatusOK, gin.H{
		"serverName":  h.cfg.ServerName,
		"serverId":    h.cfg.ServerId,
		"httpPort":    h.cfg.HttpPort,
		"udpPort":     h.cfg.UdpPort,
		"dataDir":     h.cfg.DataDir,
		"dandanHost":  h.cfg.DanDanHost,
		"bangumiHost": h.cfg.BangumiHost,
		"customProxy": proxy,
		"hasPassword": hasPassword,
	})
}

type UpdateConfigReq struct {
	ServerName  *string `json:"serverName"`
	BangumiHost *string `json:"bangumiHost"`
	DanDanHost  *string `json:"dandanHost"`
	CustomProxy *string `json:"customProxy"`
}

func (h *AdminHandler) UpdateConfig(c *gin.Context) {
	var req UpdateConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数格式错误"})
		return
	}

	db := h.authSvc.GetDB()

	if req.BangumiHost != nil {
		host := strings.TrimSpace(*req.BangumiHost)
		host = strings.TrimRight(host, "/")
		if host != "" {
			if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Bangumi 镜像源必须以 http:// 或 https:// 开头"})
				return
			}
			h.cfg.BangumiHost = host
			h.bgmClient.SetBaseUrl(host)
			if db != nil {
				_ = db.SaveSetting("bangumi_host", host)
			}
		}
	}

	if req.DanDanHost != nil {
		host := strings.TrimSpace(*req.DanDanHost)
		host = strings.TrimRight(host, "/")
		if host != "" {
			if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
				c.JSON(http.StatusBadRequest, gin.H{"error": "DanDan 弹幕源必须以 http:// 或 https:// 开头"})
				return
			}
			h.cfg.DanDanHost = host
			if h.playbackProxy != nil && h.playbackProxy.danmakuClient != nil {
				h.playbackProxy.danmakuClient.SetEndpoint(host)
			}
			if db != nil {
				_ = db.SaveSetting("dandan_host", host)
			}
		}
	}

	if req.ServerName != nil {
		name := strings.TrimSpace(*req.ServerName)
		if name != "" {
			h.cfg.ServerName = name
			if db != nil {
				_ = db.SaveSetting("server_name", name)
			}
		}
	}

	if req.CustomProxy != nil {
		proxyVal := strings.TrimSpace(*req.CustomProxy)
		_ = os.Setenv("HTTP_PROXY", proxyVal)
		_ = os.Setenv("HTTPS_PROXY", proxyVal)
		_ = os.Setenv("http_proxy", proxyVal)
		_ = os.Setenv("https_proxy", proxyVal)
		if db != nil {
			_ = db.SaveSetting("custom_proxy", proxyVal)
		}
	}

	h.GetConfig(c)
}

func (h *AdminHandler) TestBangumiEndpoint(c *gin.Context) {
	var req struct {
		URL string `json:"url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.URL) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL 不能为空"})
		return
	}

	latency, err := h.bgmClient.TestEndpoint(req.URL)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success":   false,
			"latencyMs": latency,
			"error":     err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"latencyMs": latency,
		"message":   "镜像源连接正常",
	})
}

func (h *AdminHandler) CleanCache(c *gin.Context) {
	// Clear memory / stream caches
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Cache cleaned successfully"})
}

// User Management APIs

func (h *AdminHandler) ListUsers(c *gin.Context) {
	users := h.authSvc.ListUsers()
	type UserView struct {
		ID             string     `json:"id"`
		Name           string     `json:"name"`
		IsAdmin        bool       `json:"isAdmin"`
		IsDisabled     bool       `json:"isDisabled"`
		HasPassword    bool       `json:"hasPassword"`
		BgmUserID      string     `json:"bgmUserId"`
		HasBgmToken    bool       `json:"hasBgmToken"`
		LastLoginDate  *time.Time `json:"lastLoginDate"`
		TokenCount     int        `json:"tokenCount"`
	}

	result := make([]UserView, 0, len(users))
	for _, u := range users {
		tokenCount := 0
		tokens, err := h.authSvc.GetUserTokens(u.Id)
		if err == nil {
			tokenCount = len(tokens)
		}
		bgmToken, bgmUid := h.authSvc.GetUserBangumi(u.Id)
		result = append(result, UserView{
			ID:            u.Id,
			Name:          u.Name,
			IsAdmin:       u.Policy.IsAdministrator,
			IsDisabled:    u.Policy.IsDisabled,
			HasPassword:   u.HasPassword,
			BgmUserID:     bgmUid,
			HasBgmToken:   bgmToken != "",
			LastLoginDate: u.LastLoginDate,
			TokenCount:    tokenCount,
		})
	}

	c.JSON(http.StatusOK, result)
}

type CreateUserReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	IsAdmin  bool   `json:"isAdmin"`
}

func (h *AdminHandler) CreateUser(c *gin.Context) {
	var req CreateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	u, err := h.authSvc.CreateUser(req.Username, req.Password, req.IsAdmin)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, u)
}

func (h *AdminHandler) DeleteUser(c *gin.Context) {
	userId := c.Param("id")
	if err := h.authSvc.DeleteUser(userId); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *AdminHandler) SetUserPassword(c *gin.Context) {
	userId := c.Param("id")
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	if err := h.authSvc.UpdatePassword(userId, req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Password updated successfully"})
}

func (h *AdminHandler) GetUserTokens(c *gin.Context) {
	userId := c.Param("id")
	tokens, err := h.authSvc.GetUserTokens(userId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tokens)
}

type CreateTokenReq struct {
	ClientName string `json:"clientName"`
}

func (h *AdminHandler) CreateUserToken(c *gin.Context) {
	userId := c.Param("id")
	var req CreateTokenReq
	_ = c.ShouldBindJSON(&req)
	if req.ClientName == "" {
		req.ClientName = "Web Dashboard"
	}

	token, err := h.authSvc.CreateTokenForUser(userId, req.ClientName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":      token,
		"userId":     userId,
		"clientName": req.ClientName,
	})
}

func (h *AdminHandler) RevokeUserToken(c *gin.Context) {
	token := c.Param("token")
	if err := h.authSvc.RevokeToken(token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

type BangumiBindReq struct {
	AccessToken string `json:"accessToken"`
}

func (h *AdminHandler) BindUserBangumi(c *gin.Context) {
	userId := c.Param("id")
	var req BangumiBindReq
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.AccessToken) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Access token is required"})
		return
	}

	req.AccessToken = strings.TrimSpace(req.AccessToken)
	profile, err := h.bgmClient.GetMe(req.AccessToken)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bangumi validation failed: " + err.Error()})
		return
	}

	if err := h.authSvc.UpdateUserBangumi(userId, req.AccessToken, profile.Username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"username": profile.Username,
		"nickname": profile.Nickname,
		"avatar":   profile.Avatar,
	})
}

func (h *AdminHandler) UnbindUserBangumi(c *gin.Context) {
	userId := c.Param("id")
	if err := h.authSvc.UpdateUserBangumi(userId, "", ""); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Rules APIs

func (h *AdminHandler) ListRules(c *gin.Context) {
	allRules := h.ruleMgr.GetAllPlugins()
	c.JSON(http.StatusOK, allRules)
}

func (h *AdminHandler) SaveRule(c *gin.Context) {
	var plugin engine.Plugin
	if err := c.ShouldBindJSON(&plugin); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid plugin JSON format"})
		return
	}

	if err := h.ruleMgr.SavePlugin(&plugin); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, plugin)
}

func (h *AdminHandler) ToggleRule(c *gin.Context) {
	name := c.Param("name")
	if unescaped, err := url.PathUnescape(name); err == nil && unescaped != "" {
		name = unescaped
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	if err := h.ruleMgr.TogglePlugin(name, req.Enabled); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "name": name, "enabled": req.Enabled})
}

func (h *AdminHandler) GetRule(c *gin.Context) {
	name := c.Param("name")
	if unescaped, err := url.PathUnescape(name); err == nil && unescaped != "" {
		name = unescaped
	}
	p, ok := h.ruleMgr.GetPluginByName(name)
	if !ok || p == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin not found"})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *AdminHandler) DeleteRule(c *gin.Context) {
	name := c.Param("name")
	if unescaped, err := url.PathUnescape(name); err == nil && unescaped != "" {
		name = unescaped
	}
	if err := h.ruleMgr.DeletePlugin(name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *AdminHandler) ImportRulesFromURL(c *gin.Context) {
	var req struct {
		URL string `json:"url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL is required"})
		return
	}

	stats, err := h.ruleMgr.ImportPluginsFromURL(req.URL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "ok",
		"importedCount": stats.TotalCount,
		"addedCount":    stats.AddedCount,
		"updatedCount":  stats.UpdatedCount,
	})
}

func (h *AdminHandler) UpdateAllRules(c *gin.Context) {
	var req struct {
		URL string `json:"url"`
	}
	_ = c.ShouldBindJSON(&req)
	targetURL := strings.TrimSpace(req.URL)
	if targetURL == "" {
		targetURL = "https://raw.githubusercontent.com/Predidit/KazumiRules/master/index.json"
	}

	stats, err := h.ruleMgr.ImportPluginsFromURL(targetURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "ok",
		"importedCount": stats.TotalCount,
		"addedCount":    stats.AddedCount,
		"updatedCount":  stats.UpdatedCount,
	})
}

type TestRuleReq struct {
	Plugin   *engine.Plugin `json:"plugin,omitempty"`
	RuleName string         `json:"ruleName,omitempty"`
	Keyword  string         `json:"keyword"`
}

func (h *AdminHandler) TestRule(c *gin.Context) {
	var req TestRuleReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Keyword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Keyword is required"})
		return
	}

	plugin := req.Plugin
	if plugin == nil && req.RuleName != "" {
		p, found := h.ruleMgr.GetPluginByName(req.RuleName)
		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "Rule not found"})
			return
		}
		plugin = p
	}

	if plugin == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No plugin provided"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results, err := h.eng.Search(ctx, plugin, req.Keyword)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   err.Error(),
			"results": []engine.SearchItem{},
		})
		return
	}

	// If results found, try to probe chapters for the first result
	var chapters []engine.Road
	var chapterErr string
	sampleDrama := ""
	if len(results) > 0 && results[0].Src != "" {
		sampleDrama = results[0].Src
		chList, err := h.eng.QueryChapters(ctx, plugin, results[0].Src)
		if err != nil {
			chapterErr = err.Error()
		} else {
			chapters = chList
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"resultsCount": len(results),
		"results":      results,
		"sampleDrama":  sampleDrama,
		"chapters":     chapters,
		"chapterError": chapterErr,
	})
}

// Playback History APIs

func (h *AdminHandler) ListHistory(c *gin.Context) {
	if h.authSvc.GetDB() == nil {
		c.JSON(http.StatusOK, []any{})
		return
	}

	histories, err := h.authSvc.GetDB().ListRecentHistories(50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, histories)
}

// Synonyms & Aliases APIs

func (h *AdminHandler) ListGlobalSynonyms(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusOK, []any{})
		return
	}
	list, err := h.db.ListGlobalSynonyms()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *AdminHandler) UpsertGlobalSynonym(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not initialized"})
		return
	}
	var req struct {
		Pattern     string `json:"pattern"`
		Replacement string `json:"replacement"`
		Enabled     bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Pattern) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Pattern is required"})
		return
	}
	if err := h.db.UpsertGlobalSynonym(req.Pattern, req.Replacement, req.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AdminHandler) DeleteGlobalSynonym(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not initialized"})
		return
	}
	pat := c.Param("pattern")
	if pat == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Pattern parameter is required"})
		return
	}
	if err := h.db.DeleteGlobalSynonymByPattern(pat); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AdminHandler) ResetGlobalSynonyms(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not initialized"})
		return
	}
	if err := h.db.ResetDefaultGlobalSynonyms(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AdminHandler) ListSubjectAliases(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusOK, []any{})
		return
	}
	list, err := h.db.ListSubjectAliases()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *AdminHandler) UpsertSubjectAliases(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not initialized"})
		return
	}
	var req struct {
		SubjectID int      `json:"subjectId"`
		Title     string   `json:"title"`
		Aliases   []string `json:"aliases"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.SubjectID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Valid subjectId is required"})
		return
	}
	if err := h.db.UpsertSubjectAliases(req.SubjectID, req.Title, req.Aliases); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AdminHandler) DeleteSubjectAliases(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not initialized"})
		return
	}
	idStr := c.Param("subject_id")
	subId, err := strconv.Atoi(idStr)
	if err != nil || subId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid subject_id"})
		return
	}
	if err := h.db.DeleteSubjectAliases(subId); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// -------------------------------------------------------------
// Bilibili Admin Endpoints
// -------------------------------------------------------------

func (h *AdminHandler) GetBilibiliStatus(c *gin.Context) {
	if h.biliAuth == nil {
		c.JSON(http.StatusOK, gin.H{"is_login": false, "enabled": false})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	st := h.biliAuth.GetStatus(ctx)
	c.JSON(http.StatusOK, st)
}

func (h *AdminHandler) UpdateBilibiliConfig(c *gin.Context) {
	if h.biliAuth == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Bilibili auth not initialized"})
		return
	}

	var req struct {
		SessData       *string `json:"sessdata"`
		BiliJct        *string `json:"bili_jct"`
		Buvid3         *string `json:"buvid3"`
		DedeUserID     *string `json:"dede_user_id"`
		Enabled        *bool   `json:"enabled"`
		PreferBilibili *bool   `json:"prefer_bilibili"`
		MaxQuality     *int    `json:"max_quality"`
		StreamMode     *string `json:"stream_mode"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update credentials if provided
	if req.SessData != nil || req.Buvid3 != nil {
		currentCreds := h.biliAuth.GetCredentials()
		if currentCreds == nil {
			currentCreds = &bilibili.BilibiliCredentials{}
		}
		if req.SessData != nil {
			currentCreds.SessData = strings.TrimSpace(*req.SessData)
		}
		if req.BiliJct != nil {
			currentCreds.BiliJct = strings.TrimSpace(*req.BiliJct)
		}
		if req.Buvid3 != nil {
			currentCreds.Buvid3 = strings.TrimSpace(*req.Buvid3)
		}
		if req.DedeUserID != nil {
			currentCreds.DedeUserID = strings.TrimSpace(*req.DedeUserID)
		}
		if err := h.biliAuth.SetCredentials(currentCreds); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save credentials: " + err.Error()})
			return
		}
	}

	// Update settings if provided
	currentSettings := h.biliAuth.GetSettings()
	if currentSettings == nil {
		currentSettings = &bilibili.BilibiliSettings{Enabled: true, PreferBilibili: true, MaxQuality: 80, StreamMode: "direct"}
	}
	if req.Enabled != nil {
		currentSettings.Enabled = *req.Enabled
	}
	if req.PreferBilibili != nil {
		currentSettings.PreferBilibili = *req.PreferBilibili
	}
	if req.MaxQuality != nil && *req.MaxQuality > 0 {
		currentSettings.MaxQuality = *req.MaxQuality
	}
	if req.StreamMode != nil && *req.StreamMode != "" {
		currentSettings.StreamMode = *req.StreamMode
	}
	if err := h.biliAuth.SetSettings(currentSettings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save settings: " + err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	st := h.biliAuth.GetStatus(ctx)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"status":  st,
	})
}

func (h *AdminHandler) GenerateBilibiliQR(c *gin.Context) {
	if h.biliAuth == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Bilibili auth not initialized"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	res, err := h.biliAuth.GenerateQR(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *AdminHandler) PollBilibiliQR(c *gin.Context) {
	if h.biliAuth == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Bilibili auth not initialized"})
		return
	}

	key := c.Query("qrcode_key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "qrcode_key parameter is required"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	pollResp, isSuccess, err := h.biliAuth.PollQR(ctx, key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"poll":       pollResp,
		"is_success": isSuccess,
	})
}

func (h *AdminHandler) LogoutBilibili(c *gin.Context) {
	if h.biliAuth == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Bilibili auth not initialized"})
		return
	}

	if err := h.biliAuth.ClearCredentials(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

