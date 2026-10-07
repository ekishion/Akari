package engine

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

type XPathEngine struct {
	client *http.Client
}

func NewXPathEngine() *XPathEngine {
	return &XPathEngine{
		client: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func NormalizeURL(baseURL, targetURL string) string {
	targetURL = strings.TrimSpace(targetURL)
	if targetURL == "" {
		return ""
	}
	if strings.HasPrefix(targetURL, "http://") || strings.HasPrefix(targetURL, "https://") {
		return targetURL
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return targetURL
	}
	rel, err := url.Parse(targetURL)
	if err != nil {
		return targetURL
	}
	return base.ResolveReference(rel).String()
}

func (e *XPathEngine) Search(ctx context.Context, plugin *Plugin, keyword string) ([]SearchItem, error) {
	if plugin.SearchURL == "" || plugin.SearchList == "" {
		return nil, fmt.Errorf("searchURL or searchList is empty")
	}

	searchURL := strings.ReplaceAll(plugin.SearchURL, "@keyword", url.QueryEscape(keyword))
	searchURL = NormalizeURL(plugin.BaseURL, searchURL)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}

	if plugin.UserAgent != "" {
		req.Header.Set("User-Agent", plugin.UserAgent)
	} else {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	}
	if plugin.Referer != "" {
		req.Header.Set("Referer", plugin.Referer)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, fmt.Errorf("upstream returned status code: %d", resp.StatusCode)
	}

	doc, err := htmlquery.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse html: %w", err)
	}

	nodes, err := htmlquery.QueryAll(doc, plugin.SearchList)
	if err != nil {
		return nil, fmt.Errorf("xpath query searchList failed: %w", err)
	}

	results := make([]SearchItem, 0)
	for _, node := range nodes {
		name := ""
		if plugin.SearchName != "" {
			nameNode := htmlquery.FindOne(node, plugin.SearchName)
			if nameNode != nil {
				name = strings.TrimSpace(htmlquery.InnerText(nameNode))
			}
		}

		src := ""
		if plugin.SearchResult != "" {
			srcNode := htmlquery.FindOne(node, plugin.SearchResult)
			if srcNode != nil {
				src = strings.TrimSpace(htmlquery.SelectAttr(srcNode, "href"))
			}
		}

		if name != "" && src != "" {
			results = append(results, SearchItem{
				PluginName: plugin.Name,
				Name:       name,
				Src:        NormalizeURL(plugin.BaseURL, src),
			})
		}
	}

	return results, nil
}

func (e *XPathEngine) QueryChapters(ctx context.Context, plugin *Plugin, detailURL string) ([]Road, error) {
	detailURL = NormalizeURL(plugin.BaseURL, detailURL)

	req, err := http.NewRequestWithContext(ctx, "GET", detailURL, nil)
	if err != nil {
		return nil, err
	}

	if plugin.UserAgent != "" {
		req.Header.Set("User-Agent", plugin.UserAgent)
	} else {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	}
	if plugin.Referer != "" {
		req.Header.Set("Referer", plugin.Referer)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request detail failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, fmt.Errorf("upstream detail status: %d", resp.StatusCode)
	}

	doc, err := htmlquery.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse html: %w", err)
	}

	var roadNodes []*html.Node
	if plugin.ChapterRoads != "" {
		roadNodes, err = htmlquery.QueryAll(doc, plugin.ChapterRoads)
		if err != nil {
			roadNodes = []*html.Node{doc}
		}
	} else {
		roadNodes = []*html.Node{doc}
	}

	if len(roadNodes) == 0 {
		roadNodes = []*html.Node{doc}
	}

	roads := make([]Road, 0)
	for _, rNode := range roadNodes {
		epNodes, err := htmlquery.QueryAll(rNode, plugin.ChapterResult)
		if err != nil || len(epNodes) == 0 {
			continue
		}

		var episodes []Episode
		var urls []string
		var names []string

		for epIndex, epNode := range epNodes {
			epURL := strings.TrimSpace(htmlquery.SelectAttr(epNode, "href"))
			if epURL == "" {
				continue
			}
			epName := strings.TrimSpace(htmlquery.InnerText(epNode))
			if epName == "" {
				epName = fmt.Sprintf("第%d集", epIndex+1)
			}
			fullURL := NormalizeURL(plugin.BaseURL, epURL)

			episodes = append(episodes, Episode{
				Name: epName,
				URL:  fullURL,
			})
			urls = append(urls, fullURL)
			names = append(names, epName)
		}

		if len(episodes) > 0 {
			roads = append(roads, Road{
				Name:       fmt.Sprintf("播放线路 %d", len(roads)+1),
				Episodes:   episodes,
				Identifier: names,
				Data:       urls,
			})
		}
	}

	return roads, nil
}
