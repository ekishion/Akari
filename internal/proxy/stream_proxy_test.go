package proxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"akari-bridge/internal/config"
)

func TestStreamProxy_HandleM3U8(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Mock upstream m3u8 server
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		body := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:10
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-KEY:METHOD=AES-128,URI="key.key"
#EXTINF:10.000,
segment001.ts
#EXTINF:10.000,
http://cdn.upstream.com/segment002.ts
#EXT-X-ENDLIST`
		fmt.Fprint(w, body)
	}))
	defer upstreamServer.Close()

	cfg := &config.Config{
		HttpPort: 8096,
	}
	p := NewStreamProxy(cfg, nil)

	router := gin.New()
	router.GET("/stream/m3u8", p.HandleM3U8)

	req := httptest.NewRequest(http.MethodGet, "/stream/m3u8?url="+upstreamServer.URL+"/playlist.m3u8", nil)
	req.Host = "192.168.1.2:8096"
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d, body: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	t.Logf("Rewritten M3U8:\n%s", body)

	// Verify that segment lines use the request host 192.168.1.2:8096
	if !strings.Contains(body, "http://192.168.1.2:8096/stream/segment?url=") {
		t.Errorf("Expected rewritten segment URLs to contain http://192.168.1.2:8096/stream/segment?url=")
	}

	// Verify that key line uses the request host
	if !strings.Contains(body, `URI="http://192.168.1.2:8096/stream/segment?url=`) {
		t.Errorf("Expected rewritten EXT-X-KEY to contain http://192.168.1.2:8096")
	}
}
