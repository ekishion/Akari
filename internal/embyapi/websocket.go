package embyapi

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins
	},
}

type WebSocketHandler struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]bool
}

func NewWebSocketHandler() *WebSocketHandler {
	h := &WebSocketHandler{
		clients: make(map[*websocket.Conn]bool),
	}
	go h.heartbeatLoop()
	return h
}

// HandleWebSocket handles /embywebsocket and /websocket
func (h *WebSocketHandler) HandleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[WebSocket] Upgrade failed: %v", err)
		return
	}

	h.mu.Lock()
	h.clients[conn] = true
	h.mu.Unlock()

	log.Printf("[WebSocket] Client connected: %s", conn.RemoteAddr().String())

	defer func() {
		h.mu.Lock()
		delete(h.clients, conn)
		h.mu.Unlock()
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

func (h *WebSocketHandler) Broadcast(msg any) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for client := range h.clients {
		_ = client.WriteJSON(msg)
	}
}

func (h *WebSocketHandler) heartbeatLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		h.Broadcast(map[string]any{
			"MessageType": "KeepAlive",
		})
	}
}
