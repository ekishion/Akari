package emby

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"akari-bridge/internal/config"
	"akari-bridge/internal/model"
)

func TestSystemController_PingAndInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		ServerName: "AkariTest",
		ServerId:   "akari_test_id_123",
		HttpPort:   8096,
	}

	ctrl := NewSystemController(cfg)
	r := gin.New()
	r.GET("/emby/System/Ping", ctrl.Ping)
	r.GET("/emby/System/Info/Public", ctrl.GetPublicInfo)

	// 1. Test Ping
	req := httptest.NewRequest(http.MethodGet, "/emby/System/Ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK || w.Body.String() != "\"Emby Server\"" {
		t.Fatalf("unexpected ping response: %d %s", w.Code, w.Body.String())
	}

	// 2. Test Public Info
	req = httptest.NewRequest(http.MethodGet, "/emby/System/Info/Public", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected public info status: %d", w.Code)
	}

	var info model.PublicSystemInfo
	if err := json.Unmarshal(w.Body.Bytes(), &info); err != nil {
		t.Fatalf("failed to parse public info JSON: %v", err)
	}

	if info.ServerName != "AkariTest" || info.Id != "akari_test_id_123" {
		t.Fatalf("unexpected info content: %+v", info)
	}
}
