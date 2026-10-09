package embyapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"akari-bridge/internal/engine"
)

func TestPlaybackHandler_FindEpisodeURL(t *testing.T) {
	roads := []engine.Road{
		{
			Name:       "Road 1",
			Identifier: []string{"第1集", "第2集"},
			Data:       []string{"https://example.com/play/1", "https://example.com/play/2"},
		},
		{
			Name:       "Road 2",
			Identifier: []string{"第1话", "第2话"},
			Data:       []string{"https://example.com/alt/1", "https://example.com/alt/2"},
		},
	}

	url1 := findEpisodeURL(roads, 1, 1)
	if url1 != "https://example.com/play/1" {
		t.Fatalf("expected road 1 ep 1 url, got %s", url1)
	}

	url2 := findEpisodeURL(roads, 2, 2)
	if url2 != "https://example.com/play/2" {
		t.Fatalf("expected road 1 ep 2 url, got %s", url2)
	}

	// Out of bounds returns empty string so caller can fail over to next candidate
	urlFallback := findEpisodeURL(roads, 99, 99)
	if urlFallback != "" {
		t.Fatalf("expected empty url for out-of-bounds episode, got %s", urlFallback)
	}
}

func TestPlaybackHandler_FindEpisodeURL_ExactMatching(t *testing.T) {
	roads := []engine.Road{
		{
			Name:       "Road 1",
			Identifier: []string{"第1集", "第12集"},
			Data:       []string{"https://example.com/play/1", "https://example.com/play/12"},
		},
	}

	// Requesting episode 2 should NOT match "第12集" by substring
	url2 := findEpisodeURL(roads, 2, 2)
	if url2 != "" {
		t.Fatalf("expected no match for episode 2 when only 1 and 12 exist, got %s", url2)
	}

	// Requesting episode 12 should match via label parsing
	url12 := findEpisodeURL(roads, 12, 12)
	if url12 != "https://example.com/play/12" {
		t.Fatalf("expected road 1 ep 12 url, got %s", url12)
	}

	// Multi-season cumulative index support: season ep 1 with cumulative sort 49
	roadsCumulative := []engine.Road{
		{
			Name:       "Road Cumulative",
			Identifier: []string{"第49集", "第50集"},
			Data:       []string{"https://example.com/play/49", "https://example.com/play/50"},
		},
	}
	url49 := findEpisodeURL(roadsCumulative, 1, 49)
	if url49 != "https://example.com/play/49" {
		t.Fatalf("expected cumulative sort 49 to match '第49集', got %s", url49)
	}
}

func TestPlaybackHandler_RankSearchResults(t *testing.T) {
	items := []engine.SearchItem{
		{Name: "BanG Dream! Ave Mujica", PluginName: "A", Src: "/a"},
		{Name: "BanG Dream!", PluginName: "B", Src: "/b"},
		{Name: "BanG Dream! 第二季", PluginName: "C", Src: "/c"},
		{Name: "BanG Dream! It's MyGO!!!!!", PluginName: "D", Src: "/d"},
	}

	ranked := rankSearchResults(items, "BanG Dream! It's MyGO!!!!!", false)
	if len(ranked) == 0 {
		t.Fatalf("expected at least 1 match")
	}

	// Only MyGO should be accepted and ranked first! Other seasons/spinoffs must be rejected to prevent 选A播放B!
	if ranked[0].Name != "BanG Dream! It's MyGO!!!!!" {
		t.Fatalf("expected MyGO first, got %s", ranked[0].Name)
	}
	for _, it := range ranked {
		if it.Name == "BanG Dream!" || it.Name == "BanG Dream! 第二季" {
			t.Fatalf("unrelated season should be rejected from candidates, but found: %s", it.Name)
		}
	}
}

func TestPlaybackHandler_ProgressReporting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &PlaybackHandler{}

	r := gin.New()
	r.POST("/emby/Sessions/Playing/Progress", handler.ReportPlaybackProgress)

	req := httptest.NewRequest(http.MethodPost, "/emby/Sessions/Playing/Progress", strings.NewReader(`{"ItemId":"bgm_ep_622288_1","PositionTicks":12345678,"TotalTicks":99999999}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content, got %d", w.Code)
	}
}


