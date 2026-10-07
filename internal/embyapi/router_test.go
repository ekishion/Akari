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

	// Check Favorite Items list is empty
	reqFavList2 := httptest.NewRequest(http.MethodGet, "/emby/Users/admin/Items?Filters=IsFavorite", nil)
	wFavList2 := httptest.NewRecorder()
	r.ServeHTTP(wFavList2, reqFavList2)
	if strings.Contains(wFavList2.Body.String(), "bgm_sub_622288") {
		t.Fatalf("expected bgm_sub_622288 to be removed from favorites list: %s", wFavList2.Body.String())
	}
}
