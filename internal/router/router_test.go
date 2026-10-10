package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"akari-bridge/internal/auth"
	"akari-bridge/internal/config"
	adminctrl "akari-bridge/internal/controller/admin"
	embyctrl "akari-bridge/internal/controller/emby"
	streamctrl "akari-bridge/internal/controller/stream"
	"akari-bridge/internal/rules"
)

func TestRouter_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		ServerName: "AkariRouterTest",
		ServerId:   "test_server_123",
		HttpPort:   8096,
		DataDir:    t.TempDir(),
	}

	authSvc := auth.NewAuthService(cfg, nil)
	adminAuth := auth.NewAdminAuthService(cfg, nil)
	ruleMgr := rules.NewRuleManager(cfg)

	embyItemCtrl := embyctrl.NewItemController(cfg, nil, nil, authSvc, nil)
	embyPbCtrl := embyctrl.NewPlaybackController(cfg, nil, nil, nil)
	embyUserCtrl := embyctrl.NewUserController(authSvc, nil, nil, nil, nil)
	embySysCtrl := embyctrl.NewSystemController(cfg)

	adminAuthCtrl := adminctrl.NewAuthController(adminAuth, authSvc, nil)
	adminUserCtrl := adminctrl.NewUserController(authSvc, nil, nil)
	adminRuleCtrl := adminctrl.NewRuleController(ruleMgr, nil, nil)
	adminSysCtrl := adminctrl.NewSystemController(cfg, nil, authSvc, ruleMgr, nil, nil, nil, nil, nil, nil, nil)
	adminBiliCtrl := adminctrl.NewBilibiliController(nil)

	streamProxyCtrl := streamctrl.NewProxyController(nil, nil, nil)

	embyRouter := NewEmbyRouter(authSvc, embyItemCtrl, embyPbCtrl, embyUserCtrl, embySysCtrl)
	adminRouter := NewAdminRouter(adminAuth, adminAuthCtrl, adminUserCtrl, adminRuleCtrl, adminSysCtrl, adminBiliCtrl)
	streamRouter := NewStreamRouter(streamProxyCtrl)
	rootRouter := NewRouter(cfg, embyRouter, adminRouter, streamRouter, embySysCtrl)

	engine := rootRouter.InitEngine()

	// 1. Root Ping
	req := httptest.NewRequest(http.MethodGet, "/emby/System/Ping", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected /emby/System/Ping to return 200, got %d", w.Code)
	}

	// 2. Fallback Root Ping
	req = httptest.NewRequest(http.MethodGet, "/System/Ping", nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected /System/Ping to return 200, got %d", w.Code)
	}

	// 3. Security Headers check
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("missing security headers in response")
	}
}
