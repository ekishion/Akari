package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"akari-bridge/internal/auth"
	"akari-bridge/internal/bangumi"
	"akari-bridge/internal/config"
	"akari-bridge/internal/danmaku"
	"akari-bridge/internal/domain"
	"akari-bridge/internal/rules"
)

var startTime = time.Now()

type SystemController struct {
	cfg           *config.Config
	adminSvc      domain.IAdminService
	authSvc       *auth.AuthService
	ruleMgr       *rules.RuleManager
	bgmClient     *bangumi.Client
	danmakuClient *danmaku.Client
	settingRepo   domain.ISettingRepository
	playbackRepo  domain.IPlaybackRepository
	userRepo      domain.IUserRepository
	synonymRepo   domain.ISynonymRepository
	activityRepo  domain.IActivityRepository
}

func NewSystemController(
	cfg *config.Config,
	adminSvc domain.IAdminService,
	authSvc *auth.AuthService,
	ruleMgr *rules.RuleManager,
	bgmClient *bangumi.Client,
	danmakuClient *danmaku.Client,
	settingRepo domain.ISettingRepository,
	playbackRepo domain.IPlaybackRepository,
	userRepo domain.IUserRepository,
	synonymRepo domain.ISynonymRepository,
	activityRepo domain.IActivityRepository,
) *SystemController {
	return &SystemController{
		cfg:           cfg,
		adminSvc:      adminSvc,
		authSvc:       authSvc,
		ruleMgr:       ruleMgr,
		bgmClient:     bgmClient,
		danmakuClient: danmakuClient,
		settingRepo:   settingRepo,
		playbackRepo:  playbackRepo,
		userRepo:      userRepo,
		synonymRepo:   synonymRepo,
		activityRepo:  activityRepo,
	}
}

func (ctrl *SystemController) GetStatus(c *gin.Context) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	activeRules := 0
	totalRules := 0
	if ctrl.ruleMgr != nil {
		activeRules = len(ctrl.ruleMgr.GetEnabledPlugins())
		totalRules = len(ctrl.ruleMgr.GetAllPlugins())
	}

	historyCount := 0
	if ctrl.playbackRepo != nil {
		if h, err := ctrl.playbackRepo.ListRecentHistories(100); err == nil {
			historyCount = len(h)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"serverName":     ctrl.cfg.ServerName,
		"serverId":       ctrl.cfg.ServerId,
		"version":        "1.0.0",
		"serverUrl":      ctrl.cfg.GetServerUrl(),
		"httpPort":       ctrl.cfg.HttpPort,
		"udpPort":        ctrl.cfg.UdpPort,
		"uptimeSeconds":  int64(time.Since(startTime).Seconds()),
		"activeRules":    activeRules,
		"totalRules":     totalRules,
		"goVersion":      runtime.Version(),
		"os":             runtime.GOOS,
		"arch":           runtime.GOARCH,
		"numGoroutines":  runtime.NumGoroutine(),
		"memoryAllocMB":  m.Alloc / 1024 / 1024,
		"memorySysMB":    m.Sys / 1024 / 1024,
		"dbStats":        gin.H{"historyCount": historyCount},
		"singleUserMode": len(ctrl.authSvc.ListUsers()) <= 1,
	})
}

func (ctrl *SystemController) GetConfig(c *gin.Context) {
	proxy := config.GetSystemProxy()
	hasPassword := false
	if adminUser, ok := ctrl.authSvc.GetUser("admin"); ok {
		hasPassword = adminUser.HasPassword
	}
	c.JSON(http.StatusOK, gin.H{
		"serverName":       ctrl.cfg.ServerName,
		"serverId":         ctrl.cfg.ServerId,
		"httpPort":         ctrl.cfg.HttpPort,
		"udpPort":          ctrl.cfg.UdpPort,
		"dataDir":          ctrl.cfg.DataDir,
		"dandanHost":       ctrl.cfg.DanDanHost,
		"bangumiHost":      ctrl.cfg.BangumiHost,
		"bangumiImageHost": ctrl.cfg.BangumiImageHost,
		"enableECH":        ctrl.cfg.EnableECH,
		"customProxy":      proxy,
		"hasPassword":      hasPassword,
	})
}

type UpdateConfigReq struct {
	ServerName       *string `json:"serverName"`
	BangumiHost      *string `json:"bangumiHost"`
	BangumiImageHost *string `json:"bangumiImageHost"`
	EnableECH        *bool   `json:"enableECH"`
	DanDanHost       *string `json:"dandanHost"`
	CustomProxy      *string `json:"customProxy"`
}

func (ctrl *SystemController) UpdateConfig(c *gin.Context) {
	var req UpdateConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数格式错误"})
		return
	}

	if req.BangumiHost != nil {
		host := strings.TrimSpace(*req.BangumiHost)
		host = strings.TrimRight(host, "/")
		if host != "" {
			if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Bangumi 镜像源必须以 http:// 或 https:// 开头"})
				return
			}
			ctrl.cfg.BangumiHost = host
			if ctrl.bgmClient != nil {
				ctrl.bgmClient.SetBaseUrl(host)
			}
			if ctrl.settingRepo != nil {
				_ = ctrl.settingRepo.SaveSetting("bangumi_host", host)
			}
		}
	}

	if req.BangumiImageHost != nil {
		host := strings.TrimSpace(*req.BangumiImageHost)
		host = strings.TrimRight(host, "/")
		if host != "" {
			if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Bangumi 图片镜像源必须以 http:// 或 https:// 开头"})
				return
			}
			ctrl.cfg.BangumiImageHost = host
			if ctrl.bgmClient != nil {
				ctrl.bgmClient.SetImageHost(host)
			}
			if ctrl.settingRepo != nil {
				_ = ctrl.settingRepo.SaveSetting("bangumi_image_host", host)
			}
		}
	}

	if req.EnableECH != nil {
		ctrl.cfg.EnableECH = *req.EnableECH
		if ctrl.bgmClient != nil {
			ctrl.bgmClient.SetEnableECH(*req.EnableECH)
		}
		if ctrl.settingRepo != nil {
			_ = ctrl.settingRepo.SaveSetting("enable_ech", strconv.FormatBool(*req.EnableECH))
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
			ctrl.cfg.DanDanHost = host
			if ctrl.danmakuClient != nil {
				ctrl.danmakuClient.SetEndpoint(host)
			}
			if ctrl.settingRepo != nil {
				_ = ctrl.settingRepo.SaveSetting("dandan_host", host)
			}
		}
	}

	if req.ServerName != nil {
		name := strings.TrimSpace(*req.ServerName)
		if name != "" {
			ctrl.cfg.ServerName = name
			if ctrl.settingRepo != nil {
				_ = ctrl.settingRepo.SaveSetting("server_name", name)
			}
		}
	}

	if req.CustomProxy != nil {
		proxyVal := strings.TrimSpace(*req.CustomProxy)
		_ = os.Setenv("HTTP_PROXY", proxyVal)
		_ = os.Setenv("HTTPS_PROXY", proxyVal)
		_ = os.Setenv("http_proxy", proxyVal)
		_ = os.Setenv("https_proxy", proxyVal)
		if ctrl.settingRepo != nil {
			_ = ctrl.settingRepo.SaveSetting("custom_proxy", proxyVal)
		}
	}

	ctrl.GetConfig(c)
}

func (ctrl *SystemController) TestBangumiEndpoint(c *gin.Context) {
	var req struct {
		URL string `json:"url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.URL) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL 不能为空"})
		return
	}

	latency, err := ctrl.bgmClient.TestEndpoint(req.URL)
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

func (ctrl *SystemController) TestBangumiImageEndpoint(c *gin.Context) {
	var req struct {
		URL string `json:"url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.URL) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL 不能为空"})
		return
	}

	latency, err := ctrl.bgmClient.TestImageEndpoint(req.URL)
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
		"message":   "图片镜像源连接正常",
	})
}

func (ctrl *SystemController) CleanCache(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Cache cleaned successfully"})
}

func (ctrl *SystemController) ListHistory(c *gin.Context) {
	if ctrl.playbackRepo == nil {
		c.JSON(http.StatusOK, []any{})
		return
	}

	histories, err := ctrl.playbackRepo.ListRecentHistories(50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, histories)
}

func (ctrl *SystemController) GetAuditLogs(c *gin.Context) {
	if ctrl.activityRepo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Activity repository not available"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	logs, total, err := ctrl.activityRepo.GetAuditLogs(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total": total,
		"items": logs,
	})
}

func (ctrl *SystemController) ClearAuditLogs(c *gin.Context) {
	if ctrl.activityRepo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Activity repository not available"})
		return
	}

	clientIP := c.ClientIP()
	userAgent := c.Request.UserAgent()

	if err := ctrl.activityRepo.ClearAuditLogs(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	_ = ctrl.activityRepo.RecordAuditLog("audit_cleared", clientIP, userAgent, "Audit logs cleared by admin")
	c.JSON(http.StatusOK, gin.H{"message": "Audit logs cleared"})
}

func (ctrl *SystemController) GetBannedIPs(c *gin.Context) {
	if ctrl.userRepo == nil {
		c.JSON(http.StatusOK, gin.H{"items": []any{}})
		return
	}

	bans, err := ctrl.userRepo.GetBannedIPs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": bans})
}

type UnbanRequest struct {
	IP string `json:"ip"`
}

func (ctrl *SystemController) UnbanIP(c *gin.Context) {
	var req UnbanRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.IP == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing IP parameter"})
		return
	}

	if ctrl.userRepo != nil {
		if err := ctrl.userRepo.UnbanIP(req.IP); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if ctrl.activityRepo != nil {
			_ = ctrl.activityRepo.RecordAuditLog("ip_unbanned", c.ClientIP(), c.Request.UserAgent(), fmt.Sprintf("Manually unbanned IP: %s", req.IP))
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("IP %s unbanned successfully", req.IP)})
}

// Synonyms & Aliases
func (ctrl *SystemController) ListGlobalSynonyms(c *gin.Context) {
	if ctrl.synonymRepo == nil {
		c.JSON(http.StatusOK, []any{})
		return
	}
	list, err := ctrl.synonymRepo.ListGlobalSynonyms()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (ctrl *SystemController) UpsertGlobalSynonym(c *gin.Context) {
	if ctrl.synonymRepo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Repository not initialized"})
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
	if err := ctrl.synonymRepo.UpsertGlobalSynonym(req.Pattern, req.Replacement, req.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (ctrl *SystemController) DeleteGlobalSynonym(c *gin.Context) {
	if ctrl.synonymRepo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Repository not initialized"})
		return
	}
	pat := c.Param("pattern")
	if pat == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Pattern parameter is required"})
		return
	}
	if err := ctrl.synonymRepo.DeleteGlobalSynonymByPattern(pat); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (ctrl *SystemController) ResetGlobalSynonyms(c *gin.Context) {
	if ctrl.synonymRepo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Repository not initialized"})
		return
	}
	if err := ctrl.synonymRepo.ResetDefaultGlobalSynonyms(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (ctrl *SystemController) ListSubjectAliases(c *gin.Context) {
	if ctrl.synonymRepo == nil {
		c.JSON(http.StatusOK, []any{})
		return
	}
	list, err := ctrl.synonymRepo.ListSubjectAliases()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (ctrl *SystemController) UpsertSubjectAliases(c *gin.Context) {
	if ctrl.synonymRepo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Repository not initialized"})
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
	if err := ctrl.synonymRepo.UpsertSubjectAliases(req.SubjectID, req.Title, req.Aliases); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (ctrl *SystemController) DeleteSubjectAliases(c *gin.Context) {
	if ctrl.synonymRepo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Repository not initialized"})
		return
	}
	idStr := c.Param("subject_id")
	subId, err := strconv.Atoi(idStr)
	if err != nil || subId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid subject_id"})
		return
	}
	if err := ctrl.synonymRepo.DeleteSubjectAliases(subId); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

type TelemetryData struct {
	Timestamp      string  `json:"timestamp"`
	UptimeSeconds  int64   `json:"uptimeSeconds"`
	Goroutines     int     `json:"goroutines"`
	MemoryAllocMB  float64 `json:"memoryAllocMB"`
	MemorySysMB    float64 `json:"memorySysMB"`
	ActiveSessions int     `json:"activeSessions"`
	TotalRules     int     `json:"totalRules"`
	EnabledRules   int     `json:"enabledRules"`
	RecentLog      string  `json:"recentLog,omitempty"`
}

func (ctrl *SystemController) EventsStream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Flush initial connection packet
	c.Writer.WriteString("event: connected\ndata: {\"status\":\"ok\"}\n\n")
	c.Writer.Flush()

	notify := c.Request.Context().Done()

	for {
		select {
		case <-notify:
			return
		case <-ticker.C:
			var m runtime.MemStats
			runtime.ReadMemStats(&m)

			totalRules := 0
			enabledRules := 0
			if ctrl.ruleMgr != nil {
				all := ctrl.ruleMgr.GetAllPlugins()
				totalRules = len(all)
				for _, r := range all {
					if r.Enabled {
						enabledRules++
					}
				}
			}

			data := TelemetryData{
				Timestamp:      time.Now().Format("15:04:05"),
				UptimeSeconds:  int64(time.Since(startTime).Seconds()),
				Goroutines:     runtime.NumGoroutine(),
				MemoryAllocMB:  float64(m.Alloc) / (1024 * 1024),
				MemorySysMB:    float64(m.Sys) / (1024 * 1024),
				ActiveSessions: 0,
				TotalRules:     totalRules,
				EnabledRules:   enabledRules,
			}

			payload, _ := json.Marshal(data)
			fmt.Fprintf(c.Writer, "event: telemetry\ndata: %s\n\n", payload)
			c.Writer.Flush()
		}
	}
}
