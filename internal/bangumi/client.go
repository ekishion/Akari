package bangumi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Client struct {
	baseUrl    string
	baseMu     sync.RWMutex
	httpClient *http.Client
	cacheMu    sync.RWMutex
	cache      map[string]cacheItem
	imagesMu   sync.RWMutex
	imageMap   map[int]string // subjectId -> primaryImageUrl
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
	// 1. Explicit movie platform
	if strings.Contains(plat, "剧场版") || strings.Contains(plat, "电影") || strings.Contains(plat, "movie") {
		return true
	}
	// 2. Multi-episode TV / WEB series with > 3 episodes are NEVER single movies
	if s.TotalEps > 3 || s.Eps > 3 {
		return false
	}
	// 3. TV / WEB series with more than 1 episode are TV series
	if (plat == "tv" || plat == "web") && (s.TotalEps > 1 || s.Eps > 1) {
		return false
	}
	// 4. Curated MetaTags
	for _, m := range s.MetaTags {
		if strings.Contains(m, "剧场版") || strings.Contains(m, "电影") || strings.Contains(m, "Movie") {
			return true
		}
	}
	// 5. User Tags (only if total episodes <= 3 and tag count >= 10)
	for _, t := range s.Tags {
		if (t.Name == "剧场版" || t.Name == "动画电影" || t.Name == "电影" || t.Name == "电影版") && t.Count >= 10 {
			return true
		}
	}
	// 6. Title keywords (only if total episodes <= 3)
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

	// 1. Parse Infobox (official aliases)
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

	// Return parsed aliases from Infobox
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
		baseUrl = "https://api.bgm.tv"
	}
	return &Client{
		baseUrl: baseUrl,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		cache:    make(map[string]cacheItem),
		imageMap: make(map[int]string),
	}
}

func (c *Client) GetBaseUrl() string {
	c.baseMu.RLock()
	defer c.baseMu.RUnlock()
	if c.baseUrl == "" {
		return "https://api.bgm.tv"
	}
	return c.baseUrl
}

func (c *Client) SetBaseUrl(baseUrl string) {
	c.baseMu.Lock()
	defer c.baseMu.Unlock()
	baseUrl = strings.TrimSpace(baseUrl)
	baseUrl = strings.TrimRight(baseUrl, "/")
	if baseUrl == "" {
		baseUrl = "https://api.bgm.tv"
	}
	c.baseUrl = baseUrl
}

// TestEndpoint checks if the given Bangumi endpoint (or mirror) is accessible and measures roundtrip latency.
func (c *Client) TestEndpoint(targetUrl string) (int64, error) {
	targetUrl = strings.TrimSpace(targetUrl)
	targetUrl = strings.TrimRight(targetUrl, "/")
	if targetUrl == "" {
		targetUrl = "https://api.bgm.tv"
	}
	if !strings.HasPrefix(targetUrl, "http://") && !strings.HasPrefix(targetUrl, "https://") {
		return 0, fmt.Errorf("URL 必须以 http:// 或 https:// 开头")
	}

	testClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	testUrl := fmt.Sprintf("%s/calendar", targetUrl)
	req, err := http.NewRequest(http.MethodGet, testUrl, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", BangumiUserAgent)
	req.Header.Set("Accept", "application/json")

	start := time.Now()
	resp, err := testClient.Do(req)
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
			c.imageMap[s.Id] = img
		}
	}
}

func (c *Client) GetCachedImage(subjectId int) string {
	c.imagesMu.RLock()
	defer c.imagesMu.RUnlock()
	return c.imageMap[subjectId]
}

const BangumiUserAgent = "Predidit/Kazumi/1.0.0 (Android) (https://github.com/Predidit/Kazumi)"

// GetCalendar returns weekly broadcast schedule
func (c *Client) GetCalendar() ([]BangumiSubject, error) {
	cacheKey := "calendar"
	if cached, ok := c.getCached(cacheKey); ok {
		return cached.([]BangumiSubject), nil
	}

	baseUrl := c.GetBaseUrl()

	// 1. Direct query {baseUrl}/calendar (standard Bangumi endpoint supported by official API and mirrors)
	primaryUrl := fmt.Sprintf("%s/calendar", baseUrl)
	req, err := http.NewRequest(http.MethodGet, primaryUrl, nil)
	if err == nil {
		req.Header.Set("User-Agent", BangumiUserAgent)
		req.Header.Set("Accept", "application/json")
		resp, err := c.httpClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
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
		}
	}

	// 2. Fallback to next.bgm.tv if official domain
	if baseUrl == "https://api.bgm.tv" {
		reqUrl := "https://next.bgm.tv/p1/calendar"
		req2, err2 := http.NewRequest(http.MethodGet, reqUrl, nil)
		if err2 == nil {
			req2.Header.Set("User-Agent", BangumiUserAgent)
			req2.Header.Set("Accept", "application/json")
			fastClient := &http.Client{Timeout: 3 * time.Second}
			resp2, err2 := fastClient.Do(req2)
			if err2 == nil && resp2.StatusCode == http.StatusOK {
				defer resp2.Body.Close()
				body, _ := io.ReadAll(resp2.Body)
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

	baseUrl := c.GetBaseUrl()

	// 1. Try next.bgm.tv if on official domain (with quick 3s timeout)
	if baseUrl == "https://api.bgm.tv" {
		reqUrl := fmt.Sprintf("https://next.bgm.tv/p1/trending/subjects?type=2&limit=%d&offset=%d", limit, offset)
		req, err := http.NewRequest(http.MethodGet, reqUrl, nil)
		if err == nil {
			req.Header.Set("User-Agent", BangumiUserAgent)
			req.Header.Set("Accept", "application/json")
			fastClient := &http.Client{Timeout: 3 * time.Second}
			resp, err := fastClient.Do(req)
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
	}

	// 2. Reliable fallback: use Calendar schedule items (current broadcasting hits)
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

	reqUrl := fmt.Sprintf("%s/v0/subjects/%d", c.GetBaseUrl(), subjectId)
	req, err := http.NewRequest(http.MethodGet, reqUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	var sub BangumiSubject
	if err := json.NewDecoder(resp.Body).Decode(&sub); err != nil {
		return nil, err
	}

	c.imagesMu.Lock()
	if img := sub.GetPrimaryImage(); img != "" {
		c.imageMap[sub.Id] = img
	}
	c.imagesMu.Unlock()

	c.setCache(cacheKey, &sub, 12*time.Hour)
	return &sub, nil
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
		req.Header.Set("User-Agent", BangumiUserAgent)
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
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
	req.Header.Set("User-Agent", BangumiUserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
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
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := c.httpClient.Do(req)
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
	req.Header.Set("User-Agent", BangumiUserAgent)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
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
	req.Header.Set("User-Agent", BangumiUserAgent)
	req.Header.Set("Accept", "application/json")
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	resp, err := c.httpClient.Do(req)
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

