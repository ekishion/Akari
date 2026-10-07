package danmaku

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/tidwall/gjson"
)

type DanmakuComment struct {
	Time     float64 `json:"time"`
	Type     int     `json:"type"`  // 1: 滚动, 4: 底部, 5: 顶部
	ColorInt int     `json:"color"` // 十进制 RGB
	Text     string  `json:"text"`
}

type Client struct {
	httpClient *http.Client
	endpoint   string
	endpointMu sync.RWMutex
	cacheMu    sync.RWMutex
	cache      map[string]cacheItem
}

type cacheItem struct {
	comments  []DanmakuComment
	expiresAt time.Time
}

func NewClient(endpoint string) *Client {
	endpoint = strings.TrimSpace(endpoint)
	endpoint = strings.TrimRight(endpoint, "/")
	if endpoint == "" {
		endpoint = "https://ddplay.retr0.xyz" // 默认高可用免签端点
	}
	return &Client{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		endpoint: endpoint,
		cache:    make(map[string]cacheItem),
	}
}

func (c *Client) GetEndpoint() string {
	c.endpointMu.RLock()
	defer c.endpointMu.RUnlock()
	if c.endpoint == "" {
		return "https://ddplay.retr0.xyz"
	}
	return c.endpoint
}

func (c *Client) SetEndpoint(endpoint string) {
	c.endpointMu.Lock()
	defer c.endpointMu.Unlock()
	endpoint = strings.TrimSpace(endpoint)
	endpoint = strings.TrimRight(endpoint, "/")
	if endpoint == "" {
		endpoint = "https://ddplay.retr0.xyz"
	}
	c.endpoint = endpoint
}

func (c *Client) getCached(key string) ([]DanmakuComment, bool) {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()
	item, ok := c.cache[key]
	if !ok || time.Now().After(item.expiresAt) {
		return nil, false
	}
	return item.comments, true
}

func (c *Client) setCache(key string, comments []DanmakuComment, ttl time.Duration) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	c.cache[key] = cacheItem{
		comments:  comments,
		expiresAt: time.Now().Add(ttl),
	}
}

// GetCommentsByBgmId retrieves danmaku comments using Bangumi Subject ID & episode number
func (c *Client) GetCommentsByBgmId(ctx context.Context, bgmSubjectId, epIndex int) ([]DanmakuComment, error) {
	cacheKey := fmt.Sprintf("bgm:%d:%d", bgmSubjectId, epIndex)
	if cached, ok := c.getCached(cacheKey); ok {
		return cached, nil
	}

	// 1. Direct convention episode id: "{bangumiId}{epIndex padded to 4}"
	// This is standard DanDanPlay convention for BGM mapped anime
	convEpisodeId := fmt.Sprintf("%d%04d", bgmSubjectId, epIndex)
	comments, err := c.GetCommentsByEpisodeId(ctx, convEpisodeId)
	if err == nil && len(comments) > 0 {
		c.setCache(cacheKey, comments, 2*time.Hour)
		return comments, nil
	}

	// 2. Query DanDan bangumi info by bgm id: /api/v2/bangumi/bgmtv/{id}
	reqUrl := fmt.Sprintf("%s/api/v2/bangumi/bgmtv/%d", c.GetEndpoint(), bgmSubjectId)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqUrl, nil)
	if err == nil {
		req.Header.Set("User-Agent", "akari-bridge/1.0")
		resp, err := c.httpClient.Do(req)
		if err == nil {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			dandanBangumiId := gjson.GetBytes(body, "bangumi.bangumiId").Int()
			if dandanBangumiId > 0 {
				epId := fmt.Sprintf("%d%04d", dandanBangumiId, epIndex)
				if comments, err := c.GetCommentsByEpisodeId(ctx, epId); err == nil && len(comments) > 0 {
					c.setCache(cacheKey, comments, 2*time.Hour)
					return comments, nil
				}
			}
		}
	}

	return []DanmakuComment{}, nil
}

// SearchAndGetComments searches by anime title then finds episode comments
func (c *Client) SearchAndGetComments(ctx context.Context, title string, epIndex int) ([]DanmakuComment, error) {
	cacheKey := fmt.Sprintf("search:%s:%d", title, epIndex)
	if cached, ok := c.getCached(cacheKey); ok {
		return cached, nil
	}

	cleanTitle := cleanAnimeTitle(title)
	reqUrl := fmt.Sprintf("%s/api/v2/search/episodes?anime=%s&v2=true", c.GetEndpoint(), url.QueryEscape(cleanTitle))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "akari-bridge/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	animes := gjson.GetBytes(body, "animes").Array()
	if len(animes) == 0 {
		return []DanmakuComment{}, nil
	}

	// Check episodes in first anime result
	for _, anime := range animes {
		eps := anime.Get("episodes").Array()
		for _, ep := range eps {
			epTitle := ep.Get("episodeTitle").String()
			epId := ep.Get("episodeId").String()

			// Check if episode matches
			if isEpisodeMatch(epTitle, epIndex) {
				comments, err := c.GetCommentsByEpisodeId(ctx, epId)
				if err == nil && len(comments) > 0 {
					c.setCache(cacheKey, comments, 2*time.Hour)
					return comments, nil
				}
			}
		}
	}

	return []DanmakuComment{}, nil
}

// GetCommentsByEpisodeId retrieves raw comments for an episode ID
func (c *Client) GetCommentsByEpisodeId(ctx context.Context, episodeId string) ([]DanmakuComment, error) {
	reqUrl := fmt.Sprintf("%s/api/v2/comment/%s?withRelated=true&chConvert=1", c.GetEndpoint(), episodeId)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "akari-bridge/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	commentsJSON := gjson.GetBytes(body, "comments").Array()
	result := make([]DanmakuComment, 0, len(commentsJSON))

	for _, item := range commentsJSON {
		p := item.Get("p").String()
		m := item.Get("m").String()
		if m == "" {
			continue
		}

		timeSec := 0.0
		danmakuType := 1
		colorInt := 16777215

		var sender string
		_, _ = fmt.Sscanf(p, "%f,%d,%d,%s", &timeSec, &danmakuType, &colorInt, &sender)

		result = append(result, DanmakuComment{
			Time:     timeSec,
			Type:     danmakuType,
			ColorInt: colorInt,
			Text:     m,
		})
	}

	return result, nil
}

var cleanAnimeRe = regexp.MustCompile(`(?i)(?:\[[^\]]*\]|\([^\)]*\)|（[^）]*）|第[一二三四五六七八九十\d]+[季期部分]|Season\s*\d+|1080p|720p|4k|bdrip|hdrip)`)

func cleanAnimeTitle(title string) string {
	cleaned := cleanAnimeRe.ReplaceAllString(title, " ")
	return strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(cleaned, " "))
}

func isEpisodeMatch(epTitle string, targetIndex int) bool {
	epNumStr := fmt.Sprintf("%d", targetIndex)
	return strings.Contains(epTitle, epNumStr) || strings.Contains(epTitle, fmt.Sprintf("第%d集", targetIndex)) || strings.Contains(epTitle, fmt.Sprintf("第%d话", targetIndex))
}
