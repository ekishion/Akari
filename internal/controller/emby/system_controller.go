package emby

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"akari-bridge/internal/config"
	"akari-bridge/internal/mapper"
	"akari-bridge/internal/model"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type SystemController struct {
	cfg     *config.Config
	mu      sync.Mutex
	clients map[*websocket.Conn]bool
}

func NewSystemController(cfg *config.Config) *SystemController {
	ctrl := &SystemController{
		cfg:     cfg,
		clients: make(map[*websocket.Conn]bool),
	}
	go ctrl.heartbeatLoop()
	return ctrl
}

// GetPublicInfo handles /emby/System/Info/Public
func (ctrl *SystemController) GetPublicInfo(c *gin.Context) {
	c.JSON(http.StatusOK, model.PublicSystemInfo{
		LocalAddress:            ctrl.cfg.GetBaseURLFromRequest(c.Request),
		ServerName:              ctrl.cfg.ServerName,
		Version:                 "4.8.8.0",
		Id:                      ctrl.cfg.ServerId,
		OperatingSystem:         "Linux",
		StartupWizardCompleted: true,
	})
}

// GetInfo handles /emby/System/Info
func (ctrl *SystemController) GetInfo(c *gin.Context) {
	c.JSON(http.StatusOK, model.SystemInfo{
		ServerName:              ctrl.cfg.ServerName,
		Version:                 "4.8.8.0",
		Id:                      ctrl.cfg.ServerId,
		OperatingSystem:         "Linux",
		SupportsLibraryMonitor:  true,
		WebSocketPortNumber:     ctrl.cfg.HttpPort,
		HttpServerPortNumber:    ctrl.cfg.HttpPort,
		CanSelfRestart:          false,
		CanSelfUpdate:           false,
		SupportsAutoRunAtStartup: false,
	})
}

// Ping handles /emby/System/Ping
func (ctrl *SystemController) Ping(c *gin.Context) {
	c.String(http.StatusOK, "\"Emby Server\"")
}

// GetConfiguration handles /emby/System/Configuration
func (ctrl *SystemController) GetConfiguration(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"ServerName":                     ctrl.cfg.ServerName,
		"EnableDashboardResponseCaching": true,
		"EnableUserViews":                true,
		"IsPortAuthorized":               true,
		"EnableGroupingSpannedItems":     false,
	})
}

// GetEndpoint handles /emby/System/Endpoint
func (ctrl *SystemController) GetEndpoint(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"IsLocal":     true,
		"IsInNetwork": true,
	})
}

// GetUserViews handles GET /emby/Users/:id/Views, /UserViews, /Views
func (ctrl *SystemController) GetUserViews(c *gin.Context) {
	views := mapper.CreateVirtualViews(ctrl.cfg.ServerId)
	c.JSON(http.StatusOK, model.NewQueryResult(views))
}

// GetMediaFolders handles GET /emby/Library/MediaFolders and /Library/MediaFolders
func (ctrl *SystemController) GetMediaFolders(c *gin.Context) {
	folders := mapper.CreateCollectionFolders(ctrl.cfg.ServerId)
	c.JSON(http.StatusOK, model.NewQueryResult(folders))
}

// GetGroupingOptions handles GET /Users/:id/GroupingOptions
func (ctrl *SystemController) GetGroupingOptions(c *gin.Context) {
	c.JSON(http.StatusOK, []any{})
}

// GetRootFolder handles GET /Items/Root and /Users/:id/Items/Root
func (ctrl *SystemController) GetRootFolder(c *gin.Context) {
	c.JSON(http.StatusOK, model.BaseItemDto{
		Name:                     "Root",
		ServerId:                 ctrl.cfg.ServerId,
		Id:                       "root",
		Guid:                     "root",
		Type:                     "AggregateFolder",
		IsFolder:                 true,
		LocationType:             "FileSystem",
		Path:                     "/media",
		SortName:                 "Root",
		EnableMediaSourceDisplay: true,
	})
}

// HandleWebSocket handles /embywebsocket and /websocket
func (ctrl *SystemController) HandleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[WebSocket] Upgrade failed: %v", err)
		return
	}

	ctrl.mu.Lock()
	ctrl.clients[conn] = true
	ctrl.mu.Unlock()

	log.Printf("[WebSocket] Client connected: %s", conn.RemoteAddr().String())

	defer func() {
		ctrl.mu.Lock()
		delete(ctrl.clients, conn)
		ctrl.mu.Unlock()
		_ = conn.Close()
		log.Printf("[WebSocket] Client disconnected: %s", conn.RemoteAddr().String())
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var msg map[string]any
		if err := json.Unmarshal(message, &msg); err == nil {
			msgType, _ := msg["MessageType"].(string)
			switch msgType {
			case "KeepAlive":
				_ = conn.WriteJSON(map[string]any{"MessageType": "KeepAlive"})
			case "ForceKeepAlive":
				_ = conn.WriteJSON(map[string]any{"MessageType": "ForceKeepAlive"})
			}
		}
	}
}

func (ctrl *SystemController) Broadcast(msg any) {
	ctrl.mu.Lock()
	defer ctrl.mu.Unlock()

	for client := range ctrl.clients {
		_ = client.WriteJSON(msg)
	}
}

func (ctrl *SystemController) heartbeatLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ctrl.Broadcast(map[string]any{
			"MessageType": "KeepAlive",
		})
	}
}
