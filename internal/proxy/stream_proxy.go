package proxy

import (
	"bufio"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"akari-bridge/internal/config"
	"akari-bridge/internal/engine"
	"akari-bridge/internal/resolver"
	"akari-bridge/internal/storage"
)

type StreamProxy struct {
	cfg       *config.Config
	db        *storage.DB
	client    *http.Client
	secretKey []byte
}

func NewStreamProxy(cfg *config.Config, db *storage.DB) *StreamProxy {
	secretKey := make([]byte, 32)
	if db != nil {
		if secretStr, err := db.GetSetting("stream_proxy_secret"); err == nil && secretStr != "" {
			if b, err := hex.DecodeString(secretStr); err == nil && len(b) == 32 {
				secretKey = b
			}
		}
	}
	if secretKey[0] == 0 && secretKey[31] == 0 {
		_, _ = rand.Read(secretKey)
		if db != nil {
			_ = db.SaveSetting("stream_proxy_secret", hex.EncodeToString(secretKey))
		}
	}

	return &StreamProxy{
		cfg:       cfg,
		db:        db,
		secretKey: secretKey,
		client: &http.Client{
			Transport: &http.Transport{
				Proxy: func(req *http.Request) (*url.URL, error) {
					host := strings.ToLower(req.URL.Hostname())
					if strings.HasSuffix(host, ".bilivideo.com") || strings.HasSuffix(host, ".biliapi.net") || strings.HasSuffix(host, ".hdslb.com") || host == "bilivideo.com" || host == "biliapi.net" || host == "bilibili.com" {
						return nil, nil // Direct bypass proxy
					}
					return http.ProxyFromEnvironment(req)
				},
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
				DisableKeepAlives: false,
				MaxIdleConns:      100,
				IdleConnTimeout:   90 * time.Second,
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("stopped after 10 redirects")
				}
				if len(via) > 0 {
					lastResp := via[len(via)-1].Response
					if lastResp != nil {
						refPol := strings.ToLower(lastResp.Header.Get("Referrer-Policy"))
						if strings.Contains(refPol, "no-referrer") || strings.Contains(refPol, "same-origin") {
							req.Header.Del("Referer")
						}
					}
					if via[0].URL.Host != req.URL.Host {
						req.Header.Del("Referer")
					}
				}
				return nil
			},
		},
	}
}

// HandleM3U8 rewrites the upstream m3u8 playlist so all segment & key URLs point to this proxy
func (p *StreamProxy) HandleM3U8(c *gin.Context) {
	rawTarget := strings.TrimSpace(c.Query("url"))
	if rawTarget == "" {
		c.String(http.StatusBadRequest, "Missing url parameter")
		return
	}

	u, err := url.Parse(rawTarget)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		c.String(http.StatusBadRequest, "Invalid or disallowed stream URL scheme")
		return
	}

	itemId := c.Query("item_id")
	referer := c.Query("referer")
	ua := c.Query("ua")
	if ua == "" {
		ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	}

	log.Printf("[StreamProxy] Fetching upstream M3U8: %s", rawTarget)

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, rawTarget, nil)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid request URL")
		return
	}

	req.Header.Set("User-Agent", ua)
	if referer != "" && resolver.ShouldSendReferer(rawTarget, referer) {
		req.Header.Set("Referer", referer)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		log.Printf("[StreamProxy] Error fetching upstream M3U8 (%s): %v", rawTarget, err)
		c.String(http.StatusBadGateway, "Failed to fetch upstream m3u8: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		log.Printf("[StreamProxy] Upstream M3U8 returned non-200 status: %d", resp.StatusCode)
		c.String(resp.StatusCode, "Upstream returned status: %d", resp.StatusCode)
		return
	}

	finalURL := resp.Request.URL.String()
	proxyBase := p.cfg.GetBaseURLFromRequest(c.Request)

	c.Header("Content-Type", "application/vnd.apple.mpegurl; charset=utf-8")
	c.Header("Cache-Control", "no-cache")

	scanner := bufio.NewScanner(resp.Body)
	var output strings.Builder
	var totalDuration float64
	var isVOD bool

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			output.WriteString("\n")
			continue
		}

		// Parse #EXTINF: duration
		if strings.HasPrefix(line, "#EXTINF:") {
			durStr := strings.TrimPrefix(line, "#EXTINF:")
			durStr = strings.Split(durStr, ",")[0]
			if dur, err := strconv.ParseFloat(strings.TrimSpace(durStr), 64); err == nil && dur > 0 {
				totalDuration += dur
			}
		} else if line == "#EXT-X-ENDLIST" {
			isVOD = true
		}

		// Handle EXT-X-KEY URI rewrite
		if strings.HasPrefix(line, "#EXT-X-KEY:") {
			line = p.rewriteKeyLine(line, finalURL, referer, ua, proxyBase)
			output.WriteString(line + "\n")
			continue
		}

		// Comment or tag
		if strings.HasPrefix(line, "#") {
			output.WriteString(line + "\n")
			continue
		}

		// Segment URL or sub-m3u8 playlist
		resolvedURL := engine.NormalizeURL(finalURL, line)
		if strings.Contains(strings.ToLower(resolvedURL), ".m3u8") {
			// Sub playlist
			proxyURL := fmt.Sprintf("%s/stream/m3u8?item_id=%s&url=%s&referer=%s&ua=%s",
				proxyBase, url.QueryEscape(itemId), url.QueryEscape(resolvedURL), url.QueryEscape(referer), url.QueryEscape(ua))
			output.WriteString(proxyURL + "\n")
		} else {
			// Media segment
			proxyURL := fmt.Sprintf("%s/stream/segment?url=%s&referer=%s&ua=%s",
				proxyBase, url.QueryEscape(resolvedURL), url.QueryEscape(referer), url.QueryEscape(ua))
			output.WriteString(proxyURL + "\n")
		}
	}

	if isVOD && totalDuration > 0 && itemId != "" && p.db != nil {
		ticks := int64(totalDuration * 10000000)
		_ = p.db.SaveItemDuration(itemId, ticks)
	}

	c.String(http.StatusOK, output.String())
}

// HandleSegment proxies ts/media segments with header injection and Range support
func (p *StreamProxy) HandleSegment(c *gin.Context) {
	rawTarget := strings.TrimSpace(c.Query("url"))
	if rawTarget == "" {
		c.String(http.StatusBadRequest, "Missing url parameter")
		return
	}

	u, err := url.Parse(rawTarget)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		c.String(http.StatusBadRequest, "Invalid or disallowed segment URL scheme")
		return
	}

	referer := c.Query("referer")
	ua := c.Query("ua")
	cookie := c.Query("cookie")
	if ua == "" {
		ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	}
	if referer == "" && (strings.Contains(u.Hostname(), "bilivideo.com") || strings.Contains(u.Hostname(), "biliapi.net")) {
		referer = "https://www.bilibili.com/"
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, rawTarget, nil)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid segment URL")
		return
	}

	req.Header.Set("User-Agent", ua)
	if referer != "" && resolver.ShouldSendReferer(rawTarget, referer) {
		req.Header.Set("Referer", referer)
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	// Pass Range header for seek support
	if rangeHeader := c.GetHeader("Range"); rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		log.Printf("[StreamProxy] Error proxying segment (%s): %v", rawTarget, err)
		c.String(http.StatusBadGateway, "Failed to proxy segment: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("[StreamProxy] Upstream returned status %d for segment: %s", resp.StatusCode, rawTarget)
	}

	// Pass upstream status and content headers
	if resp.Header.Get("Content-Type") != "" {
		c.Header("Content-Type", resp.Header.Get("Content-Type"))
	} else {
		c.Header("Content-Type", "video/MP2T")
	}

	if cr := resp.Header.Get("Content-Range"); cr != "" {
		c.Header("Content-Range", cr)
	}
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		c.Header("Content-Length", cl)
	}
	c.Header("Accept-Ranges", "bytes")

	c.Status(resp.StatusCode)
	_, _ = io.Copy(c.Writer, resp.Body)
}

func (p *StreamProxy) rewriteKeyLine(line, baseURL, referer, ua, proxyBase string) string {
	uriIdx := strings.Index(line, "URI=\"")
	if uriIdx == -1 {
		return line
	}
	endIdx := strings.Index(line[uriIdx+5:], "\"")
	if endIdx == -1 {
		return line
	}

	rawURI := line[uriIdx+5 : uriIdx+5+endIdx]
	resolvedURI := engine.NormalizeURL(baseURL, rawURI)
	proxyURI := fmt.Sprintf("%s/stream/segment?url=%s&referer=%s&ua=%s",
		proxyBase, url.QueryEscape(resolvedURI), url.QueryEscape(referer), url.QueryEscape(ua))

	return line[:uriIdx+5] + proxyURI + line[uriIdx+5+endIdx:]
}

func (p *StreamProxy) SignURL(targetURL string, exp time.Duration) (string, int64) {
	expUnix := time.Now().Add(exp).Unix()
	msg := fmt.Sprintf("%s:%d", targetURL, expUnix)
	mac := hmac.New(sha256.New, p.secretKey)
	mac.Write([]byte(msg))
	token := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return token, expUnix
}

func (p *StreamProxy) VerifyToken(targetURL, token string, expUnix int64) bool {
	if expUnix < time.Now().Unix() {
		return false
	}
	msg := fmt.Sprintf("%s:%d", targetURL, expUnix)
	mac := hmac.New(sha256.New, p.secretKey)
	mac.Write([]byte(msg))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(token), []byte(expected))
}

