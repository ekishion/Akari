package config

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetLocalIP(t *testing.T) {
	ip := getLocalIP()
	t.Logf("Detected local IP: %s", ip)
	if ip == "" {
		t.Fatalf("getLocalIP() returned empty string")
	}
	if strings.HasPrefix(ip, "192.168.137.") {
		t.Fatalf("getLocalIP() should not pick Windows Mobile Hotspot adapter: %s", ip)
	}
	if strings.HasPrefix(ip, "169.254.") {
		t.Fatalf("getLocalIP() should not pick link-local IP: %s", ip)
	}
	if parsed := net.ParseIP(ip); parsed == nil {
		t.Fatalf("getLocalIP() returned invalid IP: %s", ip)
	} else if ip4 := parsed.To4(); ip4 != nil {
		if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
			t.Fatalf("getLocalIP() should not pick virtual 172.x subnet: %s", ip)
		}
	}
}

func TestGetBaseURLFromRequest(t *testing.T) {
	cfg := &Config{
		HttpPort: 8096,
	}

	// 1. With standard Host header
	req1 := httptest.NewRequest(http.MethodGet, "http://192.168.1.2:8096/emby/Items/123/PlaybackInfo", nil)
	base1 := cfg.GetBaseURLFromRequest(req1)
	if base1 != "http://192.168.1.2:8096" {
		t.Errorf("Expected http://192.168.1.2:8096, got %s", base1)
	}

	// 2. With X-Forwarded-Proto https
	req2 := httptest.NewRequest(http.MethodGet, "http://myemby.example.com/emby/test", nil)
	req2.Header.Set("X-Forwarded-Proto", "https")
	base2 := cfg.GetBaseURLFromRequest(req2)
	if base2 != "https://myemby.example.com" {
		t.Errorf("Expected https://myemby.example.com, got %s", base2)
	}

	// 3. Fallback when request is nil
	base3 := cfg.GetBaseURLFromRequest(nil)
	if !strings.HasPrefix(base3, "http://") {
		t.Errorf("Expected fallback starting with http://, got %s", base3)
	}
}
