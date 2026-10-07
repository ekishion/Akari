package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

type ApiEngine struct {
	client *http.Client
}

func NewApiEngine() *ApiEngine {
	return &ApiEngine{
		client: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (e *ApiEngine) Search(ctx context.Context, plugin *Plugin, keyword string) ([]SearchItem, error) {
	apiConfig := plugin.SearchApiConfig
	if apiConfig.Request.URL == "" {
		return nil, fmt.Errorf("search API URL is empty")
	}

	searchURL := strings.ReplaceAll(apiConfig.Request.URL, "@keyword", url.QueryEscape(keyword))
	searchURL = NormalizeURL(plugin.BaseURL, searchURL)

	method := strings.ToUpper(apiConfig.Request.Method)
	if method == "" {
		method = "GET"
	}

	var bodyReader io.Reader
	if reqBody := formatRequestBody(apiConfig.Request.Body, "@keyword", keyword); reqBody != "" {
		bodyReader = strings.NewReader(reqBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, searchURL, bodyReader)
	if err != nil {
		return nil, err
	}

	for k, v := range apiConfig.Request.Headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("User-Agent") == "" {
		if plugin.UserAgent != "" {
			req.Header.Set("User-Agent", plugin.UserAgent)
		} else {
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		}
	}
	if method == "POST" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API search request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	rawJSON := string(bodyBytes)
	itemsPath := cleanJsonPath(firstNonEmpty(apiConfig.ItemsJsonPath, apiConfig.ListPath))
	namePath := cleanJsonPath(firstNonEmpty(apiConfig.NameJsonPath, apiConfig.NamePath))
	srcPath := cleanJsonPath(firstNonEmpty(apiConfig.SrcJsonPath, apiConfig.SourcePath))

	itemsResult := gjson.Get(rawJSON, itemsPath)
	if !itemsResult.IsArray() {
		return nil, fmt.Errorf("JSONPath did not resolve to array: %s", itemsPath)
	}

	var results []SearchItem
	itemsResult.ForEach(func(key, value gjson.Result) bool {
		name := value.Get(namePath).String()
		src := value.Get(srcPath).String()
		if name != "" && src != "" {
			results = append(results, SearchItem{
				PluginName: plugin.Name,
				Name:       strings.TrimSpace(name),
				Src:        strings.TrimSpace(src),
			})
		}
		return true
	})

	return results, nil
}

func (e *ApiEngine) QueryChapters(ctx context.Context, plugin *Plugin, detailURL string) ([]Road, error) {
	apiConfig := plugin.ChapterApiConfig
	if apiConfig.Request.URL == "" {
		return nil, fmt.Errorf("chapter API URL is empty")
	}

	targetURL := strings.ReplaceAll(apiConfig.Request.URL, "@source", detailURL)
	targetURL = NormalizeURL(plugin.BaseURL, targetURL)

	method := strings.ToUpper(apiConfig.Request.Method)
	if method == "" {
		method = "GET"
	}

	var bodyReader io.Reader
	if reqBody := formatRequestBody(apiConfig.Request.Body, "@source", detailURL); reqBody != "" {
		bodyReader = strings.NewReader(reqBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, targetURL, bodyReader)
	if err != nil {
		return nil, err
	}

	for k, v := range apiConfig.Request.Headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("User-Agent") == "" {
		if plugin.UserAgent != "" {
			req.Header.Set("User-Agent", plugin.UserAgent)
		} else {
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		}
	}
	if method == "POST" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("chapter API request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read chapter response: %w", err)
	}

	rawJSON := string(bodyBytes)
	roadsPath := cleanJsonPath(firstNonEmpty(apiConfig.RoadsJsonPath, apiConfig.RoadsPath))
	roadNamePath := cleanJsonPath(firstNonEmpty(apiConfig.RoadNameJsonPath, apiConfig.RoadNamePath))
	urlsPath := cleanJsonPath(firstNonEmpty(apiConfig.UrlsJsonPath, apiConfig.EpisodeUrlPath))
	namesPath := cleanJsonPath(firstNonEmpty(apiConfig.NamesJsonPath, apiConfig.EpisodeNamePath))

	roadsResult := gjson.Get(rawJSON, roadsPath)
	var roads []Road

	processEpisodes := func(roadName string, container gjson.Result) {
		var epList gjson.Result
		if apiConfig.EpisodeUrlPath != "" && strings.Contains(apiConfig.EpisodeUrlPath, "[*]") {
			epList = container.Get(cleanJsonPath(strings.Split(apiConfig.EpisodeUrlPath, "[*]")[0]))
		} else {
			epList = container.Get(urlsPath)
		}

		var episodes []Episode
		var urls []string
		var names []string

		if epList.IsArray() {
			idx := 0
			epList.ForEach(func(uKey, uVal gjson.Result) bool {
				rawEpURL := uVal.String()
				if uVal.IsObject() {
					rawEpURL = uVal.Get(cleanJsonPath(apiConfig.EpisodeUrlPath)).String()
				}
				name := fmt.Sprintf("第%d集", idx+1)
				if namesPath != "" {
					if uVal.IsObject() {
						n := uVal.Get(namesPath).String()
						if n != "" {
							name = n
						}
					}
				}

				playURL := rawEpURL
				if apiConfig.EpisodePage.URL != "" {
					t := apiConfig.EpisodePage.URL
					t = strings.ReplaceAll(t, "@source", detailURL)
					t = strings.ReplaceAll(t, "@episodeUrl", rawEpURL)
					playURL = NormalizeURL(plugin.BaseURL, t)
				} else {
					playURL = NormalizeURL(plugin.BaseURL, rawEpURL)
				}

				episodes = append(episodes, Episode{Name: name, URL: playURL})
				urls = append(urls, playURL)
				names = append(names, name)
				idx++
				return true
			})
		}

		if len(episodes) > 0 {
			roads = append(roads, Road{
				Name:       roadName,
				Episodes:   episodes,
				Identifier: names,
				Data:       urls,
			})
		}
	}

	if roadsResult.IsArray() {
		roadsResult.ForEach(func(key, value gjson.Result) bool {
			roadName := value.Get(roadNamePath).String()
			if roadName == "" {
				roadName = fmt.Sprintf("播放线路 %d", len(roads)+1)
			}
			processEpisodes(roadName, value)
			return true
		})
	} else if roadsResult.IsObject() {
		roadName := "默认线路"
		processEpisodes(roadName, roadsResult)
	}

	return roads, nil
}

func cleanJsonPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "$.")
	p = strings.TrimPrefix(p, "$")
	return p
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func formatRequestBody(body any, placeholder, value string) string {
	if body == nil {
		return ""
	}
	switch v := body.(type) {
	case string:
		return strings.ReplaceAll(v, placeholder, value)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return strings.ReplaceAll(string(b), placeholder, value)
	}
}
