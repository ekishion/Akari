package embyapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"akari-bridge/internal/auth"
	"akari-bridge/internal/bangumi"
	"akari-bridge/internal/config"
	"akari-bridge/internal/danmaku"
	"akari-bridge/internal/engine"
	"akari-bridge/internal/model"
	"akari-bridge/internal/proxy"
	"akari-bridge/internal/resolver"
	"akari-bridge/internal/rules"
	"akari-bridge/internal/storage"
)

func TestAdminRouter_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tempDir := t.TempDir()

	cfg := &config.Config{
		DataDir:       tempDir,
		ServerName:    "Kazumi Media",
		AdminUsername: "admin",
		AdminPassword: "",
		ServerId:      "testserver_adm",
		HttpPort:      8096,
		UdpPort:       7359,
		DanDanHost:    "https://api.dandanplay.net",
		BangumiHost:   "https://api.bgm.tv",
	}

	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	defer db.Close()

	authSvc := auth.NewAuthService(cfg, db)
	bgmClient := bangumi.NewClient(cfg.BangumiHost)
	danmakuClient := danmaku.NewClient(cfg.DanDanHost)
	ruleMgr := rules.NewRuleManager(cfg)
	eng := engine.NewEngine()
	res := resolver.NewStreamResolver()
	streamProxy := proxy.NewStreamProxy(cfg, db)

	router := SetupRouter(cfg, authSvc, bgmClient, ruleMgr, eng, res, streamProxy, danmakuClient, db)

	// 1. Test /api/system/status
	reqStatus := httptest.NewRequest(http.MethodGet, "/api/system/status", nil)
	wStatus := httptest.NewRecorder()
	router.ServeHTTP(wStatus, reqStatus)
	if wStatus.Code != http.StatusOK {
		t.Fatalf("expected 200 for /api/system/status, got %d", wStatus.Code)
	}

	var statusBody map[string]any
	if err := json.Unmarshal(wStatus.Body.Bytes(), &statusBody); err != nil {
		t.Fatalf("failed to decode status json: %v", err)
	}
	if statusBody["serverName"] != "Kazumi Media" {
		t.Errorf("unexpected serverName: %v", statusBody["serverName"])
	}

	// 2. Test /api/rules
	reqRules := httptest.NewRequest(http.MethodGet, "/api/rules", nil)
	wRules := httptest.NewRecorder()
	router.ServeHTTP(wRules, reqRules)
	if wRules.Code != http.StatusOK {
		t.Fatalf("expected 200 for /api/rules, got %d", wRules.Code)
	}

	// 3. Test /api/users
	reqUsers := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	wUsers := httptest.NewRecorder()
	router.ServeHTTP(wUsers, reqUsers)
	if wUsers.Code != http.StatusOK {
		t.Fatalf("expected 200 for /api/users, got %d", wUsers.Code)
	}

	// 4. Test Web SPA routing /web/
	reqWeb := httptest.NewRequest(http.MethodGet, "/web/", nil)
	wWeb := httptest.NewRecorder()
	router.ServeHTTP(wWeb, reqWeb)
	if wWeb.Code != http.StatusOK {
		t.Fatalf("expected 200 for /web/, got %d", wWeb.Code)
	}
	if !strings.Contains(wWeb.Body.String(), "<html") {
		t.Errorf("expected html content for /web/, got: %s", wWeb.Body.String())
	}

	// 5. Test root browser redirect
	reqRoot := httptest.NewRequest(http.MethodGet, "/", nil)
	reqRoot.Header.Set("Accept", "text/html,application/xhtml+xml")
	wRoot := httptest.NewRecorder()
	router.ServeHTTP(wRoot, reqRoot)
	if wRoot.Code != http.StatusTemporaryRedirect {
		t.Errorf("expected 307 redirect for browser on /, got %d", wRoot.Code)
	}

	// 6. Test FavoriteItems mark and unmark
	reqFav := httptest.NewRequest(http.MethodPost, "/emby/Users/admin/FavoriteItems/bgm_sub_622288", nil)
	wFav := httptest.NewRecorder()
	router.ServeHTTP(wFav, reqFav)
	if wFav.Code != http.StatusOK {
		t.Fatalf("expected 200 for POST FavoriteItems, got %d: %s", wFav.Code, wFav.Body.String())
	}
	var favRes model.UserItemDataDto
	_ = json.Unmarshal(wFav.Body.Bytes(), &favRes)
	if !favRes.IsFavorite {
		t.Errorf("expected IsFavorite=true, got false")
	}

	// Unmark favorite
	reqUnfav := httptest.NewRequest(http.MethodDelete, "/emby/Users/00000000000000000000000000000001/FavoriteItems/bgm_sub_622288", nil)
	wUnfav := httptest.NewRecorder()
	router.ServeHTTP(wUnfav, reqUnfav)
	if wUnfav.Code != http.StatusOK {
		t.Fatalf("expected 200 for DELETE FavoriteItems, got %d: %s", wUnfav.Code, wUnfav.Body.String())
	}
	var unfavRes model.UserItemDataDto
	_ = json.Unmarshal(wUnfav.Body.Bytes(), &unfavRes)
	if unfavRes.IsFavorite {
		t.Errorf("expected IsFavorite=false, got true")
	}
}
