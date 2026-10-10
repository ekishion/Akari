package resolver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"akari-bridge/internal/engine"
)

const defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"

type ResolvedStream struct {
	OriginalURL string `json:"originalUrl"`
	RealURL     string `json:"realUrl"`
	Format      string `json:"format"` // "m3u8" | "mp4" | "none"
	Referer     string `json:"referer"`
	UserAgent   string `json:"userAgent"`
}

type StreamResolver struct {
	client      *http.Client
	probeClient *http.Client
}

func NewStreamResolver() *StreamResolver {
	return &StreamResolver{
		client: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				Proxy:               http.ProxyFromEnvironment,
				MaxIdleConns:        16,
				IdleConnTimeout:     60 * time.Second,
				TLSHandshakeTimeout: 15 * time.Second,
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
		probeClient: &http.Client{
			Timeout: 3 * time.Second,
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout: 2 * time.Second,
				}).DialContext,
				TLSHandshakeTimeout:   2 * time.Second,
				ResponseHeaderTimeout: 2500 * time.Millisecond,
			},
		},
	}
}

// Resolve extracts the direct video stream from a web page
func (r *StreamResolver) Resolve(ctx context.Context, pageURL, referer, userAgent string) (*ResolvedStream, error) {
	pageURL = strings.TrimSpace(pageURL)
	if pageURL == "" {
		return nil, fmt.Errorf("pageURL is empty")
	}

	if userAgent == "" {
		userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
	}

	// 1. Direct media URL check
	if isDirectMediaURL(pageURL) {
		return r.buildResult(pageURL, pageURL, referer, userAgent), nil
	}

	// Also check if pageURL itself has video stream parameter (e.g. ?url=https://...m3u8)
	if extracted := decodeVideoSourceParam(pageURL); extracted != pageURL && isDirectMediaURL(extracted) {
		return r.buildResult(pageURL, extracted, referer, userAgent), nil
	}

	// 2. Fetch page HTML
	htmlContent, finalURL, isMedia, err := r.fetchHTML(ctx, pageURL, referer, userAgent)
	if err == nil {
		if isMedia {
			return r.buildResult(pageURL, finalURL, referer, userAgent), nil
		}

		currentReferer := referer
		if currentReferer == "" {
			currentReferer = finalURL
		}

		// 3. Match MacCMS player_aaaa configuration
		if streamURL, ok := r.extractMacCMS(htmlContent, finalURL); ok && streamURL != "" {
			if isDirectMediaURL(streamURL) {
				return r.buildResult(pageURL, streamURL, currentReferer, userAgent), nil
			}
			// Second hop if player_aaaa returned intermediate URL
			if subStream, ok := r.sniffSecondHop(ctx, streamURL, finalURL, userAgent); ok && subStream != "" {
				return r.buildResult(pageURL, subStream, streamURL, userAgent), nil
			}
		}

		// 4. Regex search for player configs (Artplayer / DPlayer)
		if streamURL, ok := r.extractScriptConfigs(htmlContent); ok && streamURL != "" {
			streamURL = engine.NormalizeURL(finalURL, streamURL)
			return r.buildResult(pageURL, streamURL, currentReferer, userAgent), nil
		}

		// 5. Match <video> or <source> tags
		if streamURL, ok := r.extractVideoTags(htmlContent); ok && streamURL != "" {
			streamURL = engine.NormalizeURL(finalURL, streamURL)
			return r.buildResult(pageURL, streamURL, currentReferer, userAgent), nil
		}

		// 6. Regex search for direct m3u8 or mp4
		if realURL, ok := r.extractRegexMedia(htmlContent); ok && realURL != "" {
			return r.buildResult(pageURL, realURL, currentReferer, userAgent), nil
		}

		// 7. Follow <iframe> tags if present
		if iframeSrcs := r.extractIframes(htmlContent, finalURL); len(iframeSrcs) > 0 {
			for _, iframeSrc := range iframeSrcs {
				// Check parameter in iframe src
				if extracted := decodeVideoSourceParam(iframeSrc); extracted != iframeSrc && isDirectMediaURL(extracted) {
					return r.buildResult(pageURL, extracted, finalURL, userAgent), nil
				}

				if subStream, ok := r.sniffSecondHop(ctx, iframeSrc, finalURL, userAgent); ok && subStream != "" {
					return r.buildResult(pageURL, subStream, iframeSrc, userAgent), nil
				}
			}
		}
	}

	return nil, fmt.Errorf("unable to resolve video stream from %s", pageURL)
}

func (r *StreamResolver) buildResult(originalURL, realURL, referer, userAgent string) *ResolvedStream {
	format := "mp4"
	if strings.Contains(strings.ToLower(realURL), ".m3u8") {
		format = "m3u8"
	}
	return &ResolvedStream{
		OriginalURL: originalURL,
		RealURL:     realURL,
		Format:      format,
		Referer:     referer,
		UserAgent:   userAgent,
	}
}

func (r *StreamResolver) fetchHTML(ctx context.Context, targetURL, referer, userAgent string) (string, string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return "", "", false, err
	}

	req.Header.Set("User-Agent", userAgent)
	if referer != "" {
		req.Header.Set("Referer", referer)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return "", "", false, err
	}
	defer resp.Body.Close()

	finalURL := resp.Request.URL.String()
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(contentType, "mpegurl") || strings.Contains(contentType, "video/") {
		return "", finalURL, true, nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return "", finalURL, false, fmt.Errorf("status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return "", finalURL, false, err
	}

	return string(body), finalURL, false, nil
}

var maccmsRegexList = []*regexp.Regexp{
	regexp.MustCompile(`player_aaaa\s*=\s*({[^<]+?});?<\/(?:script|div)`),
	regexp.MustCompile(`(?:var|let|const)?\s*player_aaaa\s*=\s*(\{[^<]+?\});?`),
}

func (r *StreamResolver) extractMacCMS(html, baseURL string) (string, bool) {
	var jsonStr string
	for _, re := range maccmsRegexList {
		if match := re.FindStringSubmatch(html); len(match) >= 2 {
			jsonStr = match[1]
			break
		}
	}

	if jsonStr == "" {
		return "", false
	}

	var data struct {
		URL     string `json:"url"`
		Encrypt any    `json:"encrypt"`
		From    string `json:"from"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return "", false
	}

	rawURL := strings.TrimSpace(data.URL)
	if rawURL == "" {
		return "", false
	}

	encryptMode := fmt.Sprintf("%v", data.Encrypt)
	decodedURL := decodeMacCMSUrl(rawURL, encryptMode)
	if decodedURL == "" {
		return "", false
	}

	if extracted := decodeVideoSourceParam(decodedURL); extracted != decodedURL {
		return engine.NormalizeURL(baseURL, extracted), true
	}

	return engine.NormalizeURL(baseURL, decodedURL), true
}

func decodeMacCMSUrl(raw, encrypt string) string {
	raw = strings.TrimSpace(raw)
	switch encrypt {
	case "1":
		unescaped, _ := url.QueryUnescape(raw)
		return unescaped
	case "2":
		decoded, err := base64.StdEncoding.DecodeString(raw)
		if err == nil {
			if unescaped, err := url.QueryUnescape(string(decoded)); err == nil {
				return unescaped
			}
			return string(decoded)
		}
		decoded, err = base64.URLEncoding.DecodeString(raw)
		if err == nil {
			if unescaped, err := url.QueryUnescape(string(decoded)); err == nil {
				return unescaped
			}
			return string(decoded)
		}
		return raw
	case "3":
		unescaped, _ := url.QueryUnescape(raw)
		decoded, err := base64.StdEncoding.DecodeString(unescaped)
		if err == nil {
			return string(decoded)
		}
		return unescaped
	default:
		return raw
	}
}

var (
	artplayerConfigRegex = regexp.MustCompile(`(?i)url\s*:\s*['"](https?:\/\/[^'"]+\.(?:m3u8|mp4)[^'"]*)['"]`)
	scriptMediaRegex     = regexp.MustCompile(`(?i)["'](https?:\/\/[^"'\s<>]+\.(?:m3u8|mp4)[^"'\s<>]*)["']`)
	videoTagRegex        = regexp.MustCompile(`(?i)<video\b[^>]*\bsrc=["']([^"']+)["']`)
	sourceTagRegex       = regexp.MustCompile(`(?i)<source\b[^>]*\bsrc=["']([^"']+)["']`)
	iframeTagRegex       = regexp.MustCompile(`(?i)<iframe\b[^>]*\bsrc=["']([^"']+)["']`)
	directMediaExtRegex  = regexp.MustCompile(`(?i)\.(m3u8|mp4|mkv|flv|webm)(\?.*)?$`)
)

func isAdOrPosterURL(lower string) bool {
	return strings.Contains(lower, "adposter") ||
		strings.Contains(lower, "ad_poster") ||
		strings.Contains(lower, "advert") ||
		strings.Contains(lower, "/ad/") ||
		strings.Contains(lower, "ad.mp4") ||
		strings.Contains(lower, "adv_") ||
		strings.Contains(lower, "poster.mp4") ||
		strings.Contains(lower, "loading.mp4")
}

func (r *StreamResolver) extractScriptConfigs(html string) (string, bool) {
	if m := artplayerConfigRegex.FindStringSubmatch(html); len(m) > 1 {
		if !isAdOrPosterURL(strings.ToLower(m[1])) {
			return m[1], true
		}
	}
	allMatches := scriptMediaRegex.FindAllStringSubmatch(html, -1)
	for _, m := range allMatches {
		if len(m) > 1 {
			val := m[1]
			lower := strings.ToLower(val)
			if !strings.Contains(lower, "ad.") && !strings.Contains(lower, "google") && !isAdOrPosterURL(lower) {
				return val, true
			}
		}
	}
	return "", false
}

func (r *StreamResolver) extractVideoTags(html string) (string, bool) {
	if m := videoTagRegex.FindStringSubmatch(html); len(m) > 1 {
		if !isAdOrPosterURL(strings.ToLower(m[1])) {
			return m[1], true
		}
	}
	if m := sourceTagRegex.FindStringSubmatch(html); len(m) > 1 {
		if !isAdOrPosterURL(strings.ToLower(m[1])) {
			return m[1], true
		}
	}
	return "", false
}

func (r *StreamResolver) extractRegexMedia(html string) (string, bool) {
	reM3U8 := regexp.MustCompile(`https?://[^\s"'<>\\]+?\.m3u8[^\s"'<>\\]*`)
	if match := reM3U8.FindString(html); match != "" && !isAdOrPosterURL(strings.ToLower(match)) {
		return match, true
	}

	reMP4 := regexp.MustCompile(`https?://[^\s"'<>\\]+?\.mp4[^\s"'<>\\]*`)
	allMP4 := reMP4.FindAllString(html, -1)
	for _, mp4 := range allMP4 {
		if !isAdOrPosterURL(strings.ToLower(mp4)) {
			return mp4, true
		}
	}

	return "", false
}

func (r *StreamResolver) extractIframes(html, baseURL string) []string {
	var list []string
	matches := iframeTagRegex.FindAllStringSubmatch(html, -1)
	for _, m := range matches {
		if len(m) > 1 {
			src := strings.TrimSpace(m[1])
			if src != "" && !strings.Contains(src, "about:blank") && !strings.Contains(src, "google") && !isAdOrPosterURL(strings.ToLower(src)) {
				list = append(list, engine.NormalizeURL(baseURL, src))
			}
		}
	}
	return list
}

func (r *StreamResolver) sniffSecondHop(ctx context.Context, hopURL, referer, userAgent string) (string, bool) {
	html, finalURL, isMedia, err := r.fetchHTML(ctx, hopURL, referer, userAgent)
	if err != nil || isMedia {
		if isMedia && !isAdOrPosterURL(strings.ToLower(finalURL)) {
			return finalURL, true
		}
		return "", false
	}

	if streamURL, ok := r.extractMacCMS(html, finalURL); ok && streamURL != "" && isDirectMediaURL(streamURL) {
		return streamURL, true
	}

	if streamURL, ok := r.extractScriptConfigs(html); ok && streamURL != "" {
		return engine.NormalizeURL(finalURL, streamURL), true
	}

	if streamURL, ok := r.extractVideoTags(html); ok && streamURL != "" {
		return engine.NormalizeURL(finalURL, streamURL), true
	}

	if realURL, ok := r.extractRegexMedia(html); ok && realURL != "" {
		return realURL, true
	}

	return "", false
}

func decodeVideoSourceParam(iframeUrl string) string {
	decoded, _ := url.QueryUnescape(iframeUrl)
	u, err := url.Parse(decoded)
	if err != nil {
		return iframeUrl
	}
	for _, v := range u.Query() {
		for _, val := range v {
			if isDirectMediaURL(val) {
				return val
			}
		}
	}
	return iframeUrl
}

func isDirectMediaURL(u string) bool {
	lower := strings.ToLower(u)
	if isAdOrPosterURL(lower) {
		return false
	}
	return strings.Contains(lower, ".m3u8") ||
		strings.Contains(lower, ".mp4") ||
		strings.Contains(lower, "/m3u8") ||
		directMediaExtRegex.MatchString(u)
}

// VerifyPlayable probes whether a resolved stream URL is actually servable
// and returns the URL to use. Some sources (e.g. AList "/d/" mounts) hand out direct
// URLs their upstream refuses to serve; those must fail over to the next candidate rule
// instead of failing at play time.
func (r *StreamResolver) VerifyPlayable(ctx context.Context, stream *ResolvedStream) (string, bool) {
	if stream == nil || stream.RealURL == "" {
		return "", false
	}
	if isAdOrPosterURL(strings.ToLower(stream.RealURL)) {
		return "", false
	}
	if r.probeStream(ctx, stream.RealURL, stream.Referer, stream.UserAgent, &stream.Format) {
		return stream.RealURL, true
	}
	if alt := alistProxyURL(stream.RealURL); alt != "" {
		if r.probeStream(ctx, alt, stream.Referer, stream.UserAgent, &stream.Format) {
			log.Printf("[Resolver] Direct URL refused upstream, AList /p/ proxy works: %s", alt)
			return alt, true
		}
	}
	return "", false
}

func (r *StreamResolver) probeStream(ctx context.Context, targetURL, referer, userAgent string, format *string) bool {
	probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, targetURL, nil)
	if err != nil {
		return false
	}
	if userAgent == "" {
		userAgent = defaultUserAgent
	}
	req.Header.Set("User-Agent", userAgent)
	if ShouldSendReferer(targetURL, referer) {
		req.Header.Set("Referer", referer)
	}
	req.Header.Set("Range", "bytes=0-16383")

	client := r.probeClient
	if client == nil {
		client = r.client
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[Resolver] Stream probe failed (%s): %v", targetURL, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[Resolver] Stream probe rejected (%s): upstream status %d", targetURL, resp.StatusCode)
		return false
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	if len(body) == 0 {
		log.Printf("[Resolver] Stream probe rejected (%s): empty response body", targetURL)
		return false
	}

	detectedFormat, valid := DetectContainerFormat(body, contentType)
	if !valid {
		headPreview := string(body)
		if len(headPreview) > 32 {
			headPreview = headPreview[:32]
		}
		log.Printf("[Resolver] Stream probe rejected (%s): invalid container format (content-type=%s, head=%q)", targetURL, contentType, headPreview)
		return false
	}

	if format != nil && detectedFormat != "" {
		*format = detectedFormat
	}
	return true
}

// DetectContainerFormat inspects the initial byte chunk (up to 16KB) and content-type header
// to determine the genuine container format of the media stream.
// It strictly rejects fake media, images (.webp/.png/.jpg/.gif), and HTML/JSON error payloads.
func DetectContainerFormat(data []byte, contentType string) (string, bool) {
	if len(data) == 0 {
		return "", false
	}

	ct := strings.ToLower(contentType)
	// Immediate negative checks for images and error payloads
	if strings.Contains(ct, "image/webp") || strings.Contains(ct, "image/jpeg") || strings.Contains(ct, "image/png") || strings.Contains(ct, "image/gif") {
		return "", false
	}
	if strings.Contains(ct, "text/html") || strings.Contains(ct, "application/json") || strings.Contains(ct, "application/xml") {
		return "", false
	}

	// 1. Explicitly check and reject known Image binary signatures
	// WebP: RIFF....WEBP (0x52, 0x49, 0x46, 0x46 ... 0x57, 0x45, 0x42, 0x50)
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "", false
	}
	// JPEG: \xff\xd8\xff
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return "", false
	}
	// PNG: \x89PNG\r\n\x1a\n
	if len(data) >= 8 && data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G' && data[4] == 0x0d && data[5] == 0x0a && data[6] == 0x1a && data[7] == 0x0a {
		return "", false
	}
	// GIF: GIF87a / GIF89a
	if len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a") {
		return "", false
	}

	// 2. Reject plain text / HTML / JSON error payloads
	prefixLen := len(data)
	if prefixLen > 512 {
		prefixLen = 512
	}
	trimmedStr := strings.TrimSpace(string(data[:prefixLen]))
	lowerTrimmed := strings.ToLower(trimmedStr)
	if strings.HasPrefix(lowerTrimmed, "<!doctype") ||
		strings.HasPrefix(lowerTrimmed, "<html") ||
		strings.HasPrefix(lowerTrimmed, "<script") ||
		strings.HasPrefix(lowerTrimmed, "<?xml") ||
		strings.HasPrefix(lowerTrimmed, "{\"") ||
		strings.HasPrefix(lowerTrimmed, "{\n") ||
		strings.HasPrefix(lowerTrimmed, "{\r") ||
		strings.HasPrefix(lowerTrimmed, "{ ") {
		return "", false
	}

	// 3. HLS / M3U8 Playlist
	dataStr := string(data)
	if strings.Contains(dataStr, "#EXTM3U") || strings.Contains(dataStr, "#EXT-X-") || strings.Contains(dataStr, "#EXTINF:") {
		return "m3u8", true
	}

	// 4. MP4 / ISO Base Media File Format (ISOBMFF)
	// Standard MP4 box structure: [4 bytes size][4 bytes FourCC]
	// Common starting box types: ftyp, moov, mdat, free, skip, wide, styp
	if len(data) >= 8 {
		fourcc := string(data[4:8])
		if fourcc == "ftyp" || fourcc == "moov" || fourcc == "mdat" || fourcc == "free" || fourcc == "skip" || fourcc == "wide" || fourcc == "styp" {
			return "mp4", true
		}
	}
	// Sometimes ftyp box is preceded by a small prefix or within first 128 bytes
	maxSearch := len(data)
	if maxSearch > 128 {
		maxSearch = 128
	}
	if idx := strings.Index(string(data[:maxSearch]), "ftyp"); idx >= 4 {
		return "mp4", true
	}

	// 5. MPEG-TS (Transport Stream)
	// Starts with 0x47 ('G'). Valid TS packets repeat 0x47 every 188 (or 192/204) bytes.
	if data[0] == 0x47 {
		if len(data) >= 188*2 {
			if data[188] == 0x47 || data[192] == 0x47 || data[204] == 0x47 {
				return "ts", true
			}
		} else {
			return "ts", true
		}
	} else if len(data) >= 196 && data[4] == 0x47 && data[196] == 0x47 { // 192-byte BDAV TS
		return "ts", true
	}

	// 6. Matroska / WebM
	// Starts with EBML ID: 0x1A, 0x45, 0xDF, 0xA3
	if len(data) >= 4 && data[0] == 0x1a && data[1] == 0x45 && data[2] == 0xdf && data[3] == 0xa3 {
		return "mkv", true
	}

	// 7. FLV
	// Starts with 'FLV\x01'
	if len(data) >= 4 && data[0] == 'F' && data[1] == 'L' && data[2] == 'V' && data[3] == 0x01 {
		return "flv", true
	}

	// 8. Ogg
	if len(data) >= 4 && string(data[:4]) == "OggS" {
		return "ogg", true
	}

	// 9. AVI
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "AVI " {
		return "avi", true
	}

	return "", false
}

// ShouldSendReferer checks if the referer header should be sent to the given target URL.
// Cloud storage providers, AList endpoints, and object storage reject cross-domain hotlink referers with 400/403.
func ShouldSendReferer(targetURL, referer string) bool {
	if referer == "" || targetURL == "" {
		return false
	}
	tURL, err := url.Parse(targetURL)
	if err != nil {
		return true
	}
	if strings.HasPrefix(tURL.Path, "/d/") || strings.HasPrefix(tURL.Path, "/p/") {
		return false
	}
	host := strings.ToLower(tURL.Host)
	storageKeywords := []string{
		"pan.wo.cn", "189.cn", "139.com", "aliyundrive", "alipan", "lanzou",
		"quark.cn", "baidupcs", "moedot.net", "bcebos", "myqcloud", "aliyuncs",
	}
	for _, kw := range storageKeywords {
		if strings.Contains(host, kw) {
			return false
		}
	}
	return true
}

func alistProxyURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	if strings.HasPrefix(u.Path, "/d/") {
		alt := *u
		alt.Path = "/p/" + strings.TrimPrefix(u.Path, "/d/")
		return alt.String()
	}
	return ""
}
