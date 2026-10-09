package embyapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"akari-bridge/internal/auth"
	"akari-bridge/internal/config"
	"akari-bridge/internal/rules"
	"akari-bridge/internal/storage"
)

type AdminSecurityHandler struct {
	cfg            *config.Config
	db             *storage.DB
	adminAuth      *auth.AdminAuthService
	ruleMgr        *rules.RuleManager
	playbackRecord *PlaybackHandler
	startTime      time.Time
}

func NewAdminSecurityHandler(cfg *config.Config, db *storage.DB, adminAuth *auth.AdminAuthService, ruleMgr *rules.RuleManager, pbHandler *PlaybackHandler) *AdminSecurityHandler {
	return &AdminSecurityHandler{
		cfg:            cfg,
		db:             db,
		adminAuth:      adminAuth,
		ruleMgr:        ruleMgr,
		playbackRecord: pbHandler,
		startTime:      time.Now(),
	}
}

// -------------------------------------------------------------
// Middlewares
// -------------------------------------------------------------

func (h *AdminSecurityHandler) SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Modern web security headers
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("X-Permitted-Cross-Domain-Policies", "none")
		c.Header("X-Download-Options", "noopen")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

		// Handle CORS for admin panel and Emby clients
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, HEAD")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Emby-Token, X-Emby-Authorization, X-MediaBrowser-Token, Range")
			c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges, Content-Disposition")
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func (h *AdminSecurityHandler) AdminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Allow login endpoint without auth
		if strings.HasPrefix(c.Request.URL.Path, "/api/admin/auth/login") {
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// Also check query param token for SSE / websocket
			authHeader = c.Query("token")
		} else {
			authHeader = strings.TrimPrefix(authHeader, "Bearer ")
			authHeader = strings.TrimSpace(authHeader)
		}

		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: missing admin token"})
			return
		}

		claims, err := h.adminAuth.ValidateToken(authHeader)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: invalid or expired token"})
			return
		}

		c.Set("admin_username", claims.Username)
		c.Next()
	}
}

// -------------------------------------------------------------
// Auth Endpoints
// -------------------------------------------------------------

func (h *AdminSecurityHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	clientIP := c.ClientIP()
	userAgent := c.Request.UserAgent()

	token, expiresAt, err := h.adminAuth.Login(req.Username, req.Password, clientIP, userAgent)
	if err != nil {
		if strings.Contains(err.Error(), "locked") {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":     token,
		"expiresAt": expiresAt.Format(time.RFC3339),
		"username":  req.Username,
	})
}

func (h *AdminSecurityHandler) Logout(c *gin.Context) {
	username, _ := c.Get("admin_username")
	clientIP := c.ClientIP()
	userAgent := c.Request.UserAgent()

	if h.db != nil {
		_ = h.db.RecordAuditLog("logout", clientIP, userAgent, fmt.Sprintf("Admin '%v' logged out", username))
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

func (h *AdminSecurityHandler) GetProfile(c *gin.Context) {
	username, _ := c.Get("admin_username")
	c.JSON(http.StatusOK, gin.H{
		"username": username,
		"role":     "administrator",
	})
}

type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

func (h *AdminSecurityHandler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters"})
		return
	}

	usernameVal, _ := c.Get("admin_username")
	username, ok := usernameVal.(string)
	if !ok || username == "" {
		username = "admin"
	}

	clientIP := c.ClientIP()
	userAgent := c.Request.UserAgent()

	if err := h.adminAuth.ChangePassword(username, req.OldPassword, req.NewPassword, clientIP, userAgent); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}

// -------------------------------------------------------------
// Security & Audit Log Endpoints
// -------------------------------------------------------------

func (h *AdminSecurityHandler) GetAuditLogs(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not available"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	logs, total, err := h.db.GetAuditLogs(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total": total,
		"items": logs,
	})
}

func (h *AdminSecurityHandler) ClearAuditLogs(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not available"})
		return
	}

	clientIP := c.ClientIP()
	userAgent := c.Request.UserAgent()

	if err := h.db.ClearAuditLogs(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	_ = h.db.RecordAuditLog("audit_cleared", clientIP, userAgent, "Audit logs cleared by admin")
	c.JSON(http.StatusOK, gin.H{"message": "Audit logs cleared"})
}

func (h *AdminSecurityHandler) GetBannedIPs(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusOK, gin.H{"items": []any{}})
		return
	}

	bans, err := h.db.GetBannedIPs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": bans})
}

type UnbanRequest struct {
	IP string `json:"ip"`
}

func (h *AdminSecurityHandler) UnbanIP(c *gin.Context) {
	var req UnbanRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.IP == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing IP parameter"})
		return
	}

	if h.db != nil {
		if err := h.db.UnbanIP(req.IP); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		_ = h.db.RecordAuditLog("ip_unbanned", c.ClientIP(), c.Request.UserAgent(), fmt.Sprintf("Manually unbanned IP: %s", req.IP))
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("IP %s unbanned successfully", req.IP)})
}

// -------------------------------------------------------------
// SSE Telemetry Stream Endpoint (/api/admin/events)
// -------------------------------------------------------------

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

func (h *AdminSecurityHandler) EventsStream(c *gin.Context) {
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
			if h.ruleMgr != nil {
				all := h.ruleMgr.GetAllPlugins()
				totalRules = len(all)
				for _, r := range all {
					if r.Enabled {
						enabledRules++
					}
				}
			}

			activeSessions := 0
			if h.playbackRecord != nil {
				// Count active stream sessions in memory
				activeSessions = h.playbackRecord.getActiveSessionCount()
			}

			data := TelemetryData{
				Timestamp:      time.Now().Format("15:04:05"),
				UptimeSeconds:  int64(time.Since(h.startTime).Seconds()),
				Goroutines:     runtime.NumGoroutine(),
				MemoryAllocMB:  float64(m.Alloc) / (1024 * 1024),
				MemorySysMB:    float64(m.Sys) / (1024 * 1024),
				ActiveSessions: activeSessions,
				TotalRules:     totalRules,
				EnabledRules:   enabledRules,
			}

			payload, _ := json.Marshal(data)
			fmt.Fprintf(c.Writer, "event: telemetry\ndata: %s\n\n", payload)
			c.Writer.Flush()
		}
	}
}

// Helper to count active streams
func (h *PlaybackHandler) getActiveSessionCount() int {
	h.cacheMu.RLock()
	defer h.cacheMu.RUnlock()
	return len(h.cache)
}
