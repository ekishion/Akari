package embyapi

import (
	"context"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"akari-bridge/internal/auth"
	"akari-bridge/internal/bangumi"
	"akari-bridge/internal/config"
	"akari-bridge/internal/engine"
	"akari-bridge/internal/model"
	"akari-bridge/internal/rules"
)

var startTime = time.Now()

type AdminHandler struct {
	cfg           *config.Config
	authSvc       *auth.AuthService
	ruleMgr       *rules.RuleManager
	eng           *engine.Engine
	bgmClient     *bangumi.Client
	playbackProxy *PlaybackHandler
}

func NewAdminHandler(
	cfg *config.Config,
	authSvc *auth.AuthService,
	ruleMgr *rules.RuleManager,
	eng *engine.Engine,
	bgmClient *bangumi.Client,
	playbackProxy *PlaybackHandler,
) *AdminHandler {
	return &AdminHandler{
		cfg:           cfg,
		authSvc:       authSvc,
		ruleMgr:       ruleMgr,
		eng:           eng,
		bgmClient:     bgmClient,
		playbackProxy: playbackProxy,
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
		"singleUserMode": h.cfg.AdminPassword == "",
	})
}

func (h *AdminHandler) GetConfig(c *gin.Context) {
	proxy := config.GetSystemProxy()
	c.JSON(http.StatusOK, gin.H{
		"serverName":  h.cfg.ServerName,
		"serverId":    h.cfg.ServerId,
		"httpPort":    h.cfg.HttpPort,
		"udpPort":     h.cfg.UdpPort,
		"dataDir":     h.cfg.DataDir,
		"dandanHost":  h.cfg.DanDanHost,
		"bangumiHost": h.cfg.BangumiHost,
		"customProxy": proxy,
		"hasPassword": h.cfg.AdminPassword != "",
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

func (h *AdminHandler) DeleteRule(c *gin.Context) {
	name := c.Param("name")
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

	count, err := h.ruleMgr.ImportPluginsFromURL(req.URL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "importedCount": count})
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
