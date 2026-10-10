package bangumi

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	BangumiUserAgent  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"
	DefaultAPIBase    = "https://api.bgm.tv"
	DefaultImageBase  = "https://lain.bgm.tv"
)

type Client struct {
	baseUrl    string
	baseMu     sync.RWMutex
	imageHost  string
	imgMu      sync.RWMutex
	enableECH  bool
	echMu      sync.RWMutex
	echCacheMu sync.RWMutex
	echCache   map[string]echCacheItem
	httpClient *http.Client
	cacheMu    sync.RWMutex
	cache      map[string]cacheItem
	imagesMu   sync.RWMutex
	imageMap   map[int]string // subjectId -> primaryImageUrl
}

type echCacheItem struct {
	config    []byte
	expiresAt time.Time
}

type cacheItem struct {
	data      any
	expiresAt time.Time
}

type BangumiSubject struct {
	Id          int               `json:"id"`
	Type        int               `json:"type"`
	Name        string            `json:"name"`
	NameCn      string            `json:"name_cn"`
	Summary     string            `json:"summary"`
	AirDate     string            `json:"date"`
	AirWeekday  int               `json:"air_weekday"`
	RatingScore float64           `json:"score"`
	Rank        int               `json:"rank"`
	Platform    string            `json:"platform"`
	Eps         int               `json:"eps"`
	MetaTags    []string          `json:"meta_tags"`
	Images      map[string]string `json:"images"`
	Tags        []SubjectTag      `json:"tags"`
	Infobox     []InfoboxItem     `json:"infobox"`
	TotalEps    int               `json:"total_episodes"`
}

type InfoboxItem struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

type subjectAlias BangumiSubject

func (s *BangumiSubject) UnmarshalJSON(data []byte) error {
	type rawSubject struct {
		subjectAlias
		NameCN  string `json:"nameCN"`
		AirDate string `json:"air_date"`
	}
	var r rawSubject
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	*s = BangumiSubject(r.subjectAlias)
	if s.NameCn == "" && r.NameCN != "" {
		s.NameCn = r.NameCN
	}
	if s.AirDate == "" && r.AirDate != "" {
		s.AirDate = r.AirDate
	}
	if s.TotalEps == 0 && s.Eps > 0 {
		s.TotalEps = s.Eps
	}
	return nil
}

func (s *BangumiSubject) IsMovie() bool {
	if s == nil {
		return false
	}
	plat := strings.ToLower(s.Platform)
	if strings.Contains(plat, "剧场版") || strings.Contains(plat, "电影") || strings.Contains(plat, "movie") {
		return true
	}
	if s.TotalEps > 3 || s.Eps > 3 {
		return false
	}
	if (plat == "tv" || plat == "web") && (s.TotalEps > 1 || s.Eps > 1) {
		return false
	}
	for _, m := range s.MetaTags {
		if strings.Contains(m, "剧场版") || strings.Contains(m, "电影") || strings.Contains(m, "Movie") {
			return true
		}
	}
	for _, t := range s.Tags {
		if (t.Name == "剧场版" || t.Name == "动画电影" || t.Name == "电影" || t.Name == "电影版") && t.Count >= 10 {
			return true
		}
	}
	name := strings.ToLower(s.Name + " " + s.NameCn)
	if strings.Contains(name, "剧场版") || strings.Contains(name, "动画电影") || strings.Contains(name, "大电影") || strings.Contains(name, "the movie") {
		return true
	}
	return false
}

type SubjectTag struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func (s *BangumiSubject) ExtractAliases() []string {
	seen := make(map[string]bool)
	var aliases []string

	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" || seen[strings.ToLower(v)] {
			return
		}
		if strings.EqualFold(v, s.Name) || strings.EqualFold(v, s.NameCn) {
			return
		}
		seen[strings.ToLower(v)] = true
		aliases = append(aliases, v)
	}

	for _, info := range s.Infobox {
		key := strings.ToLower(strings.TrimSpace(info.Key))
		if key == "别名" || key == "alias" || key == "aliases" || key == "又名" || key == "中文名" || key == "英文名" || key == "日文名" {
			if len(info.Value) == 0 {
				continue
			}
			var strVal string
			if err := json.Unmarshal(info.Value, &strVal); err == nil {
				add(strVal)
				continue
			}
			var objArr []struct {
				V string `json:"v"`
			}
			if err := json.Unmarshal(info.Value, &objArr); err == nil {
				for _, o := range objArr {
					add(o.V)
				}
				continue
			}
			var strArr []string
			if err := json.Unmarshal(info.Value, &strArr); err == nil {
				for _, str := range strArr {
					add(str)
				}
				continue
			}
		}
	}

	return aliases
}

type BangumiEpisode struct {
	Id          int     `json:"id"`
	Type        int     `json:"type"` // 0=ep, 1=sp
	Sort        float64 `json:"sort"`
	Ep          int     `json:"ep"`
	Name        string  `json:"name"`
	NameCn      string  `json:"name_cn"`
	Duration    string  `json:"duration"`
	AirDate     string  `json:"airdate"`
	Description string  `json:"desc"`
}

type episodeAlias BangumiEpisode

func (e *BangumiEpisode) UnmarshalJSON(data []byte) error {
	type rawEpisode struct {
		episodeAlias
		Date        string `json:"date"`
		AirDate2    string `json:"air_date"`
		NameCN      string `json:"nameCN"`
		Description string `json:"description"`
	}
	var r rawEpisode
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	*e = BangumiEpisode(r.episodeAlias)
	if e.AirDate == "" {
		if r.AirDate2 != "" {
			e.AirDate = r.AirDate2
		} else if r.Date != "" {
			e.AirDate = r.Date
		}
	}
	if e.NameCn == "" && r.NameCN != "" {
		e.NameCn = r.NameCN
	}
	if e.Description == "" && r.Description != "" {
		e.Description = r.Description
	}
	return nil
}

func NewClient(baseUrl string) *Client {
	baseUrl = strings.TrimSpace(baseUrl)
	baseUrl = strings.TrimRight(baseUrl, "/")
	if baseUrl == "" {
		baseUrl = DefaultAPIBase
	}
	c := &Client{
		baseUrl:   baseUrl,
		imageHost: DefaultImageBase,
		enableECH: true,
		echCache:  make(map[string]echCacheItem),
		cache:     make(map[string]cacheItem),
		imageMap:  make(map[int]string),
	}
	c.httpClient = c.createHTTPClient()
	return c
}

func (c *Client) createHTTPClient() *http.Client {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
	}

	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		TLSClientConfig: tlsConfig,
		DisableKeepAlives: false,
		MaxIdleConns:      50,
		IdleConnTimeout:   90 * time.Second,
	}

	return &http.Client{
		Transport: tr,
		Timeout:   10 * time.Second,
	}
}

func (c *Client) GetBaseUrl() string {
	c.baseMu.RLock()
	defer c.baseMu.RUnlock()
	if c.baseUrl == "" {
		return DefaultAPIBase
	}
	return c.baseUrl
}

func (c *Client) SetBaseUrl(baseUrl string) {
	c.baseMu.Lock()
	defer c.baseMu.Unlock()
	baseUrl = strings.TrimSpace(baseUrl)
	baseUrl = strings.TrimRight(baseUrl, "/")
	if baseUrl == "" {
		baseUrl = DefaultAPIBase
	}
	c.baseUrl = baseUrl
}

func (c *Client) GetImageHost() string {
	c.imgMu.RLock()
	defer c.imgMu.RUnlock()
	if c.imageHost == "" {
		return DefaultImageBase
	}
	return c.imageHost
}

func (c *Client) SetImageHost(imageHost string) {
	c.imgMu.Lock()
	defer c.imgMu.Unlock()
	imageHost = strings.TrimSpace(imageHost)
	imageHost = strings.TrimRight(imageHost, "/")
	if imageHost == "" {
		imageHost = DefaultImageBase
	}
	c.imageHost = imageHost
}

func (c *Client) GetEnableECH() bool {
	c.echMu.RLock()
	defer c.echMu.RUnlock()
	return c.enableECH
}

func (c *Client) SetEnableECH(enable bool) {
	c.echMu.Lock()
	defer c.echMu.Unlock()
	c.enableECH = enable
}

// RewriteImageUrl maps official Bangumi image URLs (lain.bgm.tv) to custom image host if configured
func (c *Client) RewriteImageUrl(imgUrl string) string {
	imgUrl = strings.TrimSpace(imgUrl)
	if imgUrl == "" {
		return ""
	}

	imgHost := c.GetImageHost()
	if strings.HasPrefix(imgUrl, "http://lain.bgm.tv") {
		if imgHost != "" && imgHost != "http://lain.bgm.tv" && imgHost != "https://lain.bgm.tv" {
			return strings.Replace(imgUrl, "http://lain.bgm.tv", imgHost, 1)
		}
		return strings.Replace(imgUrl, "http://lain.bgm.tv", "https://lain.bgm.tv", 1)
	}
	if strings.HasPrefix(imgUrl, "https://lain.bgm.tv") {
		if imgHost != "" && imgHost != "http://lain.bgm.tv" && imgHost != "https://lain.bgm.tv" {
			return strings.Replace(imgUrl, "https://lain.bgm.tv", imgHost, 1)
		}
		return imgUrl
	}
	return imgUrl
}

// FetchECHConfig retrieves ECH public key from DNS Type 65 (HTTPS) via DoH
func (c *Client) FetchECHConfig(ctx context.Context, domain string) []byte {
	if !c.GetEnableECH() {
		return nil
	}

	c.echCacheMu.RLock()
	if item, ok := c.echCache[domain]; ok && time.Now().Before(item.expiresAt) {
		c.echCacheMu.RUnlock()
		return item.config
	}
	c.echCacheMu.RUnlock()

	dohURL := fmt.Sprintf("https://1.1.1.1/dns-query?name=%s&type=HTTPS", domain)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dohURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Accept", "application/dns-json")

	dohClient := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
		Timeout: 3 * time.Second,
	}
	resp, err := dohClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	var doh struct {
		Answer []struct {
			Type int    `json:"type"`
			Data string `json:"data"`
		} `json:"Answer"`
	}
	if err := json.Unmarshal(body, &doh); err != nil {
		return nil
	}

	for _, ans := range doh.Answer {
		if ans.Type == 65 {
			parts := strings.Fields(ans.Data)
			for _, p := range parts {
				if strings.HasPrefix(p, "ech=") {
					rawECH := strings.TrimPrefix(p, "ech=")
					if b, err := base64.StdEncoding.DecodeString(rawECH); err == nil && len(b) > 0 {
						c.echCacheMu.Lock()
						c.echCache[domain] = echCacheItem{
							config:    b,
							expiresAt: time.Now().Add(12 * time.Hour),
						}
						c.echCacheMu.Unlock()
						return b
					}
				}
			}
		}
	}
	return nil
}

// DoRequest executes an HTTP request with automatic ECH injection, browser headers, and retry fallback
func (c *Client) DoRequest(req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", BangumiUserAgent)
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json, text/plain, */*")
	}
	if req.Header.Get("Referer") == "" {
		req.Header.Set("Referer", "https://bgm.tv")
	}

	host := req.URL.Hostname()
	echConfig := c.FetchECHConfig(req.Context(), host)

	client := c.httpClient
	if len(echConfig) > 0 {
		tlsConfig := &tls.Config{
			MinVersion:                     tls.VersionTLS13,
			EncryptedClientHelloConfigList: echConfig,
		}
		tr := &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: tlsConfig,
		}
		client = &http.Client{
			Transport: tr,
			Timeout:   c.httpClient.Timeout,
		}
	}

	return client.Do(req)
}

// TestEndpoint checks if the given Bangumi API endpoint is accessible
func (c *Client) TestEndpoint(targetUrl string) (int64, error) {
	targetUrl = strings.TrimSpace(targetUrl)
	targetUrl = strings.TrimRight(targetUrl, "/")
	if targetUrl == "" {
		targetUrl = DefaultAPIBase
	}
	if !strings.HasPrefix(targetUrl, "http://") && !strings.HasPrefix(targetUrl, "https://") {
		return 0, fmt.Errorf("URL 必须以 http:// 或 https:// 开头")
	}

	testUrl := fmt.Sprintf("%s/calendar", targetUrl)
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, testUrl, nil)
	if err != nil {
		return 0, err
	}

	start := time.Now()
	resp, err := c.DoRequest(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return latency, fmt.Errorf("连接超时或失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return latency, fmt.Errorf("返回状态码异常: %d", resp.StatusCode)
	}

	return latency, nil
}

// TestImageEndpoint checks if the Bangumi Image mirror is accessible
func (c *Client) TestImageEndpoint(targetUrl string) (int64, error) {
	targetUrl = strings.TrimSpace(targetUrl)
	targetUrl = strings.TrimRight(targetUrl, "/")
	if targetUrl == "" {
		targetUrl = DefaultImageBase
	}

	testImgUrl := fmt.Sprintf("%s/pic/cover/l/1c/b4/390200_1IA55.jpg", targetUrl)
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, testImgUrl, nil)
	if err != nil {
		return 0, err
	}

	start := time.Now()
	resp, err := c.DoRequest(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return latency, fmt.Errorf("图片源连接失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return latency, fmt.Errorf("图片源返回状态码异常: %d", resp.StatusCode)
	}

	return latency, nil
}

func (c *Client) getCached(key string) (any, bool) {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()
	item, ok := c.cache[key]
	if !ok || time.Now().After(item.expiresAt) {
		return nil, false
	}
	return item.data, true
}

func (c *Client) setCache(key string, data any, ttl time.Duration) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	c.cache[key] = cacheItem{
		data:      data,
		expiresAt: time.Now().Add(ttl),
	}
}

func (c *Client) cacheImages(subjects []BangumiSubject) {
	c.imagesMu.Lock()
	defer c.imagesMu.Unlock()
	for _, s := range subjects {
		img := s.GetPrimaryImage()
		if img != "" {
			c.imageMap[s.Id] = c.RewriteImageUrl(img)
		}
	}
}

func (c *Client) GetCachedImage(subjectId int) string {
	c.imagesMu.RLock()
	defer c.imagesMu.RUnlock()
	return c.imageMap[subjectId]
}

// GetCalendar returns weekly broadcast schedule
func (c *Client) GetCalendar() ([]BangumiSubject, error) {
	cacheKey := "calendar"
	if cached, ok := c.getCached(cacheKey); ok {
		return cached.([]BangumiSubject), nil
	}

	candidates := []string{
		c.GetBaseUrl(),
		"https://next.bgm.tv",
		DefaultAPIBase,
	}

	seenBase := make(map[string]bool)

	for _, base := range candidates {
		base = strings.TrimRight(base, "/")
		if seenBase[base] {
			continue
		}
		seenBase[base] = true

		var reqUrl string
		if strings.Contains(base, "next.bgm.tv") {
			reqUrl = "https://next.bgm.tv/p1/calendar"
		} else {
			reqUrl = fmt.Sprintf("%s/calendar", base)
		}

		req, err := http.NewRequest(http.MethodGet, reqUrl, nil)
		if err != nil {
			continue
		}

		resp, err := c.DoRequest(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil {
				resp.Body.Close()
			}
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Try standard format
		var days []struct {
			Weekday struct {
				Id int `json:"id"`
			} `json:"weekday"`
			Items []BangumiSubject `json:"items"`
		}
		if err := json.Unmarshal(body, &days); err == nil && len(days) > 0 {
			var result []BangumiSubject
			for _, d := range days {
				for _, sub := range d.Items {
					sub.AirWeekday = d.Weekday.Id
					result = append(result, sub)
				}
			}
			if len(result) > 0 {
				c.cacheImages(result)
				c.setCache(cacheKey, result, 2*time.Hour)
				return result, nil
			}
		}

		// Try next.bgm.tv format
		var rawCalendar map[string][]struct {
			Subject BangumiSubject `json:"subject"`
		}
		if err := json.Unmarshal(body, &rawCalendar); err == nil && len(rawCalendar) > 0 {
			var result []BangumiSubject
			for day := 1; day <= 7; day++ {
				key := fmt.Sprintf("%d", day)
				if list, exists := rawCalendar[key]; exists {
					for _, item := range list {
						sub := item.Subject
						sub.AirWeekday = day
						result = append(result, sub)
					}
				}
			}
			if len(result) > 0 {
				c.cacheImages(result)
				c.setCache(cacheKey, result, 2*time.Hour)
				return result, nil
			}
		}
	}

	return nil, fmt.Errorf("all calendar endpoints failed")
}

// GetTrending returns current popular anime subjects
func (c *Client) GetTrending(limit, offset int) ([]BangumiSubject, error) {
	cacheKey := fmt.Sprintf("trending:%d:%d", limit, offset)
	if cached, ok := c.getCached(cacheKey); ok {
		return cached.([]BangumiSubject), nil
	}

	reqUrl := fmt.Sprintf("https://next.bgm.tv/p1/trending/subjects?type=2&limit=%d&offset=%d", limit, offset)
	req, err := http.NewRequest(http.MethodGet, reqUrl, nil)
	if err == nil {
		resp, err := c.DoRequest(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			var raw struct {
				Data []struct {
					Subject BangumiSubject `json:"subject"`
				} `json:"data"`
			}
			if err := json.Unmarshal(body, &raw); err == nil && len(raw.Data) > 0 {
				result := make([]BangumiSubject, 0, len(raw.Data))
				for _, item := range raw.Data {
					result = append(result, item.Subject)
				}
				c.cacheImages(result)
				c.setCache(cacheKey, result, 1*time.Hour)
				return result, nil
			}
		}
	}

	// Fallback to Calendar schedule items
	cal, err := c.GetCalendar()
	if err == nil && len(cal) > 0 {
		if len(cal) > limit {
			cal = cal[:limit]
		}
		c.setCache(cacheKey, cal, 1*time.Hour)
		return cal, nil
	}

	return nil, fmt.Errorf("all trending endpoints failed")
}

// GetSubject retrieves single subject details
func (c *Client) GetSubject(subjectId int) (*BangumiSubject, error) {
	cacheKey := fmt.Sprintf("subject:%d", subjectId)
	if cached, ok := c.getCached(cacheKey); ok {
		return cached.(*BangumiSubject), nil
	}

	candidates := []string{c.GetBaseUrl(), "https://next.bgm.tv", DefaultAPIBase}
	seen := make(map[string]bool)

	for _, base := range candidates {
		base = strings.TrimRight(base, "/")
		if seen[base] {
			continue
		}
		seen[base] = true

		reqUrl := fmt.Sprintf("%s/v0/subjects/%d", base, subjectId)
		req, err := http.NewRequest(http.MethodGet, reqUrl, nil)
		if err != nil {
			continue
		}

		resp, err := c.DoRequest(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil {
				if resp.StatusCode == http.StatusNotFound {
					resp.Body.Close()
					return nil, nil
				}
				resp.Body.Close()
			}
			continue
		}

		var sub BangumiSubject
		if err := json.NewDecoder(resp.Body).Decode(&sub); err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		c.imagesMu.Lock()
		if img := sub.GetPrimaryImage(); img != "" {
			c.imageMap[sub.Id] = c.RewriteImageUrl(img)
		}
		c.imagesMu.Unlock()

		c.setCache(cacheKey, &sub, 12*time.Hour)
		return &sub, nil
	}

	return nil, fmt.Errorf("failed to retrieve subject %d from all endpoints", subjectId)
}

// GetEpisodes retrieves all episodes for a subject
func (c *Client) GetEpisodes(subjectId int) ([]BangumiEpisode, error) {
	cacheKey := fmt.Sprintf("episodes:%d", subjectId)
	if cached, ok := c.getCached(cacheKey); ok {
		return cached.([]BangumiEpisode), nil
	}

	var allEpisodes []BangumiEpisode
	limit := 100
	offset := 0

	for {
		reqUrl := fmt.Sprintf("%s/v0/episodes?subject_id=%d&limit=%d&offset=%d", c.GetBaseUrl(), subjectId, limit, offset)
		req, err := http.NewRequest(http.MethodGet, reqUrl, nil)
		if err != nil {
			return nil, err
		}

		resp, err := c.DoRequest(req)
		if err != nil {
			return nil, err
		}

		var page struct {
			Total int              `json:"total"`
			Data  []BangumiEpisode `json:"data"`
		}
		err = json.NewDecoder(resp.Body).Decode(&page)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		if len(page.Data) == 0 {
			break
		}
		allEpisodes = append(allEpisodes, page.Data...)
		offset += len(page.Data)
		if offset >= page.Total {
			break
		}
	}

	c.setCache(cacheKey, allEpisodes, 6*time.Hour)
	return allEpisodes, nil
}

// GetEpisode retrieves single episode by episode ID and returns episode info + subjectId
func (c *Client) GetEpisode(epId int) (*BangumiEpisode, int, error) {
	cacheKey := fmt.Sprintf("episode:%d", epId)
	type cachedEp struct {
		Ep        *BangumiEpisode
		SubjectId int
	}
	if cached, ok := c.getCached(cacheKey); ok {
		item := cached.(cachedEp)
		return item.Ep, item.SubjectId, nil
	}

	reqUrl := fmt.Sprintf("%s/v0/episodes/%d", c.GetBaseUrl(), epId)
	req, err := http.NewRequest(http.MethodGet, reqUrl, nil)
	if err != nil {
		return nil, 0, err
	}

	resp, err := c.DoRequest(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	var raw struct {
		BangumiEpisode
		SubjectId int `json:"subject_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, 0, err
	}

	ep := &raw.BangumiEpisode
	c.setCache(cacheKey, cachedEp{Ep: ep, SubjectId: raw.SubjectId}, 12*time.Hour)
	return ep, raw.SubjectId, nil
}

// Search searches subjects by keyword
func (c *Client) Search(keyword string, limit int) ([]BangumiSubject, error) {
	searchPayload := map[string]any{
		"keyword": keyword,
		"sort":    "match",
		"filter": map[string]any{
			"type": []int{2}, // 2 = Anime
		},
	}

	bodyBytes, _ := json.Marshal(searchPayload)
	reqUrl := fmt.Sprintf("%s/v0/search/subjects?limit=%d&offset=0", c.GetBaseUrl(), limit)
	req, err := http.NewRequest(http.MethodPost, reqUrl, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.DoRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []BangumiSubject `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	c.cacheImages(result.Data)
	return result.Data, nil
}

func (s *BangumiSubject) GetDisplayName() string {
	if s.NameCn != "" {
		return s.NameCn
	}
	return s.Name
}

func (s *BangumiSubject) GetPrimaryImage() string {
	if s.Images != nil {
		if img, ok := s.Images["large"]; ok && img != "" {
			return img
		}
		if img, ok := s.Images["common"]; ok && img != "" {
			return img
		}
	}
	return ""
}

func BuildImageUrl(imgUrl string) string {
	if imgUrl == "" {
		return ""
	}
	return fmt.Sprintf("/emby/items/images/proxy?url=%s", url.QueryEscape(imgUrl))
}

type BangumiUserProfile struct {
	ID        int               `json:"id"`
	Username  string            `json:"username"`
	Nickname  string            `json:"nickname"`
	Avatar    map[string]string `json:"avatar"`
	Sign      string            `json:"sign"`
	UserGroup int               `json:"user_group"`
}

// GetMe retrieves the authenticated user's Bangumi profile using their Personal Access Token
func (c *Client) GetMe(accessToken string) (*BangumiUserProfile, error) {
	if accessToken == "" {
		return nil, fmt.Errorf("access token is empty")
	}

	reqUrl := fmt.Sprintf("%s/v0/me", c.GetBaseUrl())
	req, err := http.NewRequest(http.MethodGet, reqUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.DoRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bangumi auth error (status %d): %s", resp.StatusCode, string(body))
	}

	var profile BangumiUserProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

// GetUserCollections retrieves user's collections (1=Wish, 2=Watched, 3=Watching)
func (c *Client) GetUserCollections(usernameOrId string, accessToken string, collectionType int, limit, offset int) ([]BangumiSubject, error) {
	if usernameOrId == "" {
		return nil, fmt.Errorf("username or user ID is empty")
	}
	if limit <= 0 {
		limit = 30
	}

	cacheKey := fmt.Sprintf("user_col:%s:%d:%d:%d", usernameOrId, collectionType, limit, offset)
	if cached, ok := c.getCached(cacheKey); ok {
		return cached.([]BangumiSubject), nil
	}

	reqUrl := fmt.Sprintf("%s/v0/users/%s/collections?subject_type=2&type=%d&limit=%d&offset=%d", c.GetBaseUrl(), url.PathEscape(usernameOrId), collectionType, limit, offset)
	req, err := http.NewRequest(http.MethodGet, reqUrl, nil)
	if err != nil {
		return nil, err
	}
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	resp, err := c.DoRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bangumi collections error (status %d): %s", resp.StatusCode, string(body))
	}

	var raw struct {
		Total int `json:"total"`
		Data  []struct {
			Subject BangumiSubject `json:"subject"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	subjects := make([]BangumiSubject, 0, len(raw.Data))
	for _, item := range raw.Data {
		subjects = append(subjects, item.Subject)
	}

	c.cacheImages(subjects)
	c.setCache(cacheKey, subjects, 10*time.Minute)
	return subjects, nil
}
