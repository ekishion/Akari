package engine

import (
	"context"
	"strings"
	"sync"
)

type Engine struct {
	xpathEngine *XPathEngine
	apiEngine   *ApiEngine
}

func NewEngine() *Engine {
	return &Engine{
		xpathEngine: NewXPathEngine(),
		apiEngine:   NewApiEngine(),
	}
}

// Search executes single plugin search
func (e *Engine) Search(ctx context.Context, plugin *Plugin, keyword string) ([]SearchItem, error) {
	if strings.EqualFold(plugin.SearchMode, "api") {
		return e.apiEngine.Search(ctx, plugin, keyword)
	}
	return e.xpathEngine.Search(ctx, plugin, keyword)
}

// QueryChapters executes chapter resolution
func (e *Engine) QueryChapters(ctx context.Context, plugin *Plugin, detailURL string) ([]Road, error) {
	if strings.EqualFold(plugin.ChapterMode, "api") {
		return e.apiEngine.QueryChapters(ctx, plugin, detailURL)
	}
	return e.xpathEngine.QueryChapters(ctx, plugin, detailURL)
}

// SearchMulti concurrently searches all enabled plugins
func (e *Engine) SearchMulti(ctx context.Context, plugins []*Plugin, keyword string) []SearchItem {
	var wg sync.WaitGroup
	var mu sync.Mutex
	allResults := make([]SearchItem, 0)

	for _, p := range plugins {
		if !p.Enabled {
			continue
		}
		wg.Add(1)
		go func(plugin *Plugin) {
			defer wg.Done()
			items, err := e.Search(ctx, plugin, keyword)
			if err == nil && len(items) > 0 {
				mu.Lock()
				allResults = append(allResults, items...)
				mu.Unlock()
			}
		}(p)
	}

	wg.Wait()
	return allResults
}
