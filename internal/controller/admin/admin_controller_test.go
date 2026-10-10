package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"akari-bridge/internal/auth"
	"akari-bridge/internal/config"
	"akari-bridge/internal/rules"
)

func TestAdminController_RulesAndConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		ServerName: "AkariTest",
		ServerId:   "akari_admin_test",
		HttpPort:   8096,
		DataDir:    t.TempDir(),
	}

	authSvc := auth.NewAuthService(cfg, nil)
	ruleMgr := rules.NewRuleManager(cfg)
	ruleCtrl := NewRuleController(ruleMgr, nil, nil)
	sysCtrl := NewSystemController(cfg, nil, authSvc, ruleMgr, nil, nil, nil, nil, nil, nil, nil)

	r := gin.New()
	r.GET("/api/rules", ruleCtrl.ListRules)
	r.GET("/api/system/config", sysCtrl.GetConfig)

	// Test List Rules
	req := httptest.NewRequest(http.MethodGet, "/api/rules", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected rules response status: %d", w.Code)
	}

	// Test Get Config
	req = httptest.NewRequest(http.MethodGet, "/api/system/config", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected config response status: %d", w.Code)
	}

	var res map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to unmarshal config json: %v", err)
	}

	if res["serverName"] != "AkariTest" {
		t.Fatalf("unexpected config serverName: %v", res["serverName"])
	}
}
