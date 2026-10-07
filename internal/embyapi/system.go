package embyapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"akari-bridge/internal/config"
	"akari-bridge/internal/model"
)

type SystemHandler struct {
	cfg *config.Config
}

func NewSystemHandler(cfg *config.Config) *SystemHandler {
	return &SystemHandler{cfg: cfg}
}

// GetPublicInfo handles /emby/System/Info/Public
func (h *SystemHandler) GetPublicInfo(c *gin.Context) {
	c.JSON(http.StatusOK, model.PublicSystemInfo{
		LocalAddress:            h.cfg.GetBaseURLFromRequest(c.Request),
		ServerName:              h.cfg.ServerName,
		Version:                 "4.8.8.0",
		Id:                      h.cfg.ServerId,
		OperatingSystem:         "Linux",
		StartupWizardCompleted: true,
	})
}

// GetInfo handles /emby/System/Info
func (h *SystemHandler) GetInfo(c *gin.Context) {
	c.JSON(http.StatusOK, model.SystemInfo{
		ServerName:              h.cfg.ServerName,
		Version:                 "4.8.8.0",
		Id:                      h.cfg.ServerId,
		OperatingSystem:         "Linux",
		SupportsLibraryMonitor:  true,
		WebSocketPortNumber:     h.cfg.HttpPort,
		HttpServerPortNumber:    h.cfg.HttpPort,
		CanSelfRestart:          false,
		CanSelfUpdate:           false,
		SupportsAutoRunAtStartup: false,
	})
}

// Ping handles /emby/System/Ping
func (h *SystemHandler) Ping(c *gin.Context) {
	c.String(http.StatusOK, "\"Emby Server\"")
}

// GetConfiguration handles /emby/System/Configuration
func (h *SystemHandler) GetConfiguration(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"ServerName":                     h.cfg.ServerName,
		"EnableDashboardResponseCaching": true,
		"EnableUserViews":                true,
		"IsPortAuthorized":               true,
		"EnableGroupingSpannedItems":     false,
	})
}

// GetEndpoint handles /emby/System/Endpoint
func (h *SystemHandler) GetEndpoint(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"IsLocal": true,
		"IsInNetwork": true,
	})
}
