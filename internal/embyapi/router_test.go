package embyapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"akari-bridge/internal/auth"
	"akari-bridge/internal/bangumi"
	"akari-bridge/internal/config"
	"akari-bridge/internal/danmaku"
	"akari-bridge/internal/engine"
	"akari-bridge/internal/proxy"
	"akari-bridge/internal/resolver"
	"akari-bridge/internal/rules"
	"akari-bridge/internal/storage"
)

func TestSetupRouter(t *testing.T) {
	cfg := &config.Config{
		ServerName:  "Test Server",
		ServerId:    "test_server_id",
		HttpPort:    8096,
		UdpPort:     7359,
		DataDir:     t.TempDir(),
		BangumiHost: "https://api.bgm.tv",
		DanDanHost:  "https://api.dandanplay.net",
	}

	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	defer db.Close()

	authSvc := auth.NewAuthService(cfg, db)
	bgmClient := bangumi.NewClient(cfg.BangumiHost)
	ruleMgr := rules.NewRuleManager(cfg)
	eng := engine.NewEngine()
	res := resolver.NewStreamResolver()
	streamProxy := proxy.NewStreamProxy(cfg, db)
	danmakuClient := danmaku.NewClient(cfg.DanDanHost)

	// This should not panic
	r := SetupRouter(cfg, authSvc, bgmClient, ruleMgr, eng, res, streamProxy, danmakuClient, db)
	if r == nil {
		t.Fatal("expected non-nil router")
	}

	// Test GET /emby/Users/:id/Items/bgm_sub_622288
	req := httptest.NewRequest(http.MethodGet, "/emby/Users/00000000000000000000000000000001/Items/bgm_sub_622288", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Test GET /emby/Shows/bgm_sub_622288/Seasons
	reqSeasons := httptest.NewRequest(http.MethodGet, "/emby/Shows/bgm_sub_622288/Seasons", nil)
	wSeasons := httptest.NewRecorder()
	r.ServeHTTP(wSeasons, reqSeasons)
	if wSeasons.Code != http.StatusOK {
		t.Fatalf("expected 200 for Seasons, got %d: %s", wSeasons.Code, wSeasons.Body.String())
	}

	// Test GET /emby/Shows/bgm_sub_622288/Episodes
	reqEpisodes := httptest.NewRequest(http.MethodGet, "/emby/Shows/bgm_sub_622288/Episodes", nil)
	wEpisodes := httptest.NewRecorder()
	r.ServeHTTP(wEpisodes, reqEpisodes)
	if wEpisodes.Code != http.StatusOK {
		t.Fatalf("expected 200 for Episodes, got %d: %s", wEpisodes.Code, wEpisodes.Body.String())
	}

	// Test GET /emby/Users/:id/Items/bgm_ep_622288_1
	reqEp := httptest.NewRequest(http.MethodGet, "/emby/Users/00000000000000000000000000000001/Items/bgm_ep_622288_1", nil)
	wEp := httptest.NewRecorder()
	r.ServeHTTP(wEp, reqEp)
	if wEp.Code != http.StatusOK {
		t.Fatalf("expected 200 for Episode item, got %d: %s", wEp.Code, wEp.Body.String())
	}
	bodyStr := wEp.Body.String()
	if !strings.Contains(bodyStr, "MediaSources") || !strings.Contains(bodyStr, "MediaStreams") {
		t.Fatalf("expected MediaSources and MediaStreams in Episode item: %s", bodyStr)
	}

	// Test GET /emby/Items/bgm_ep_622288_1/Images/Primary
	reqImg := httptest.NewRequest(http.MethodGet, "/emby/Items/bgm_ep_622288_1/Images/Primary", nil)
	wImg := httptest.NewRecorder()
	r.ServeHTTP(wImg, reqImg)
	if wImg.Code != http.StatusOK {
		t.Fatalf("expected 200 for Episode Primary Image, got %d", wImg.Code)
	}

	// 5. Test Favorite Flow: Mark favorite on bgm_sub_622288
	reqFav := httptest.NewRequest(http.MethodPost, "/emby/Users/admin/FavoriteItems/bgm_sub_622288", nil)
	wFav := httptest.NewRecorder()
	r.ServeHTTP(wFav, reqFav)
	if wFav.Code != http.StatusOK {
		t.Fatalf("expected 200 for MarkFavorite, got %d: %s", wFav.Code, wFav.Body.String())
	}

	// Check Favorite Items list
	reqFavList := httptest.NewRequest(http.MethodGet, "/emby/Users/admin/Items?Filters=IsFavorite", nil)
	wFavList := httptest.NewRecorder()
	r.ServeHTTP(wFavList, reqFavList)
	if wFavList.Code != http.StatusOK {
		t.Fatalf("expected 200 for Favorite Items list, got %d: %s", wFavList.Code, wFavList.Body.String())
	}
	if !strings.Contains(wFavList.Body.String(), "bgm_sub_622288") {
		t.Fatalf("expected bgm_sub_622288 in favorites list: %s", wFavList.Body.String())
	}

	// Unmark favorite
	reqUnfav := httptest.NewRequest(http.MethodDelete, "/emby/Users/admin/FavoriteItems/bgm_sub_622288", nil)
	wUnfav := httptest.NewRecorder()
	r.ServeHTTP(wUnfav, reqUnfav)
	if wUnfav.Code != http.StatusOK {
		t.Fatalf("expected 200 for UnmarkFavorite, got %d", wUnfav.Code)
	}

	// 6. Test GET /emby/Users/admin/Views
	reqViews := httptest.NewRequest(http.MethodGet, "/emby/Users/admin/Views?IncludeExternalContent=false&IncludeHidden=false", nil)
	wViews := httptest.NewRecorder()
	r.ServeHTTP(wViews, reqViews)
	if wViews.Code != http.StatusOK {
		t.Fatalf("expected 200 for Views, got %d: %s", wViews.Code, wViews.Body.String())
	}
	viewsBody := wViews.Body.String()
	if !strings.Contains(viewsBody, "view_schedule") || !strings.Contains(viewsBody, "FileSystem") {
		t.Fatalf("expected view_schedule and FileSystem in Views response: %s", viewsBody)
	}

	// 7. Test GET /emby/Users/admin/Items/Resume (Must return [] not null)
	reqResume := httptest.NewRequest(http.MethodGet, "/emby/Users/admin/Items/Resume", nil)
	wResume := httptest.NewRecorder()
	r.ServeHTTP(wResume, reqResume)
	if wResume.Code != http.StatusOK {
		t.Fatalf("expected 200 for Resume, got %d: %s", wResume.Code, wResume.Body.String())
	}
	resumeBody := wResume.Body.String()
	if strings.Contains(resumeBody, `"Items":null`) {
		t.Fatalf("Resume response must not contain Items:null: %s", resumeBody)
	}
	if !strings.Contains(resumeBody, `"Items":[]`) {
		t.Fatalf("Resume response must contain Items:[]: %s", resumeBody)
	}

	// 8. Test GET /emby/System/Configuration
	reqSysCfg := httptest.NewRequest(http.MethodGet, "/emby/System/Configuration", nil)
	wSysCfg := httptest.NewRecorder()
	r.ServeHTTP(wSysCfg, reqSysCfg)
	if wSysCfg.Code != http.StatusOK {
		t.Fatalf("expected 200 for System/Configuration, got %d", wSysCfg.Code)
	}
	sysCfgBody := wSysCfg.Body.String()
	if !strings.Contains(sysCfgBody, "EnableUserViews") {
		t.Fatalf("expected EnableUserViews in System/Configuration: %s", sysCfgBody)
	}

	// 9. Test GET /emby/DisplayPreferences/usersettings
	reqDispPref := httptest.NewRequest(http.MethodGet, "/emby/DisplayPreferences/usersettings", nil)
	wDispPref := httptest.NewRecorder()
	r.ServeHTTP(wDispPref, reqDispPref)
	if wDispPref.Code != http.StatusOK {
		t.Fatalf("expected 200 for DisplayPreferences, got %d", wDispPref.Code)
	}
	if !strings.Contains(wDispPref.Body.String(), "CustomPrefs") {
		t.Fatalf("expected CustomPrefs in DisplayPreferences: %s", wDispPref.Body.String())
	}
}
