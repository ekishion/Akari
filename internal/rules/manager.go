package rules

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"akari-bridge/internal/config"
	"akari-bridge/internal/engine"
)

type RuleManager struct {
	cfg     *config.Config
	mu      sync.RWMutex
	plugins map[string]*engine.Plugin
}

func NewRuleManager(cfg *config.Config) *RuleManager {
	mgr := &RuleManager{
		cfg:     cfg,
		plugins: make(map[string]*engine.Plugin),
	}
	mgr.loadOrInitRules()
	return mgr
}

func (m *RuleManager) loadOrInitRules() {
	m.mu.Lock()
	defer m.mu.Unlock()

	pluginFile := filepath.Join(m.cfg.DataDir, "plugins.json")
	if data, err := os.ReadFile(pluginFile); err == nil {
		var list []*engine.Plugin
		if err := json.Unmarshal(data, &list); err == nil && len(list) > 0 {
			for _, p := range list {
				m.plugins[p.Name] = p
			}
			log.Printf("[RuleManager] Loaded %d rules from %s", len(m.plugins), pluginFile)
			return
		}
	}

	log.Println("[RuleManager] No rules configured. Users can import rules via Web Dashboard.")
}

func (m *RuleManager) GetAllPlugins() []*engine.Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*engine.Plugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		list = append(list, p)
	}
	return list
}

func (m *RuleManager) GetEnabledPlugins() []*engine.Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*engine.Plugin
	for _, p := range m.plugins {
		if p.Enabled {
			result = append(result, p)
		}
	}
	return result
}

func (m *RuleManager) GetPluginByName(name string) (*engine.Plugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.plugins[name]
	if !ok {
		// Try by ID
		for _, pl := range m.plugins {
			if strings.EqualFold(pl.ID, name) || strings.EqualFold(pl.Name, name) {
				return pl, true
			}
		}
	}
	return p, ok
}

func (m *RuleManager) SavePlugin(p *engine.Plugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if p.Name == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}
	if p.ID == "" {
		p.ID = "rule_" + strings.ToLower(p.Name)
	}

	m.plugins[p.Name] = p
	m.saveRulesToFileLocked()
	log.Printf("[RuleManager] Saved plugin '%s' (enabled=%v)", p.Name, p.Enabled)
	return nil
}

func (m *RuleManager) TogglePlugin(nameOrId string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var target *engine.Plugin
	for _, p := range m.plugins {
		if strings.EqualFold(p.Name, nameOrId) || strings.EqualFold(p.ID, nameOrId) {
			target = p
			break
		}
	}

	if target == nil {
		return fmt.Errorf("plugin '%s' not found", nameOrId)
	}

	target.Enabled = enabled
	m.saveRulesToFileLocked()
	log.Printf("[RuleManager] Toggled plugin '%s' -> enabled=%v", target.Name, enabled)
	return nil
}

func (m *RuleManager) DeletePlugin(nameOrId string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var targetKey string
	for k, p := range m.plugins {
		if strings.EqualFold(p.Name, nameOrId) || strings.EqualFold(p.ID, nameOrId) {
			targetKey = k
			break
		}
	}

	if targetKey == "" {
		return fmt.Errorf("plugin '%s' not found", nameOrId)
	}

	delete(m.plugins, targetKey)
	m.saveRulesToFileLocked()
	log.Printf("[RuleManager] Deleted plugin '%s'", targetKey)
	return nil
}

func (m *RuleManager) ImportPluginsJSON(data []byte, sourceBaseURL string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. Try single plugin
	var single engine.Plugin
	if err := json.Unmarshal(data, &single); err == nil && single.Name != "" && single.BaseURL != "" {
		if single.ID == "" {
			single.ID = "rule_" + strings.ToLower(single.Name)
		}
		single.Enabled = true
		m.plugins[single.Name] = &single
		m.saveRulesToFileLocked()
		return 1, nil
	}

	// 2. Try array of full plugins or index.json
	var rawList []map[string]any
	if err := json.Unmarshal(data, &rawList); err == nil && len(rawList) > 0 {
		var fullPlugins []*engine.Plugin
		var indexNames []string

		for _, item := range rawList {
			itemBytes, _ := json.Marshal(item)
			var p engine.Plugin
			if err := json.Unmarshal(itemBytes, &p); err == nil && p.Name != "" {
				if p.BaseURL != "" || p.SearchURL != "" {
					if p.ID == "" {
						p.ID = "rule_" + strings.ToLower(p.Name)
					}
					fullPlugins = append(fullPlugins, &p)
				} else {
					// Index-only entry (e.g. Predidit/KazumiRules index.json)
					indexNames = append(indexNames, p.Name)
				}
			}
		}

		importedCount := len(fullPlugins)
		// Save all full plugins found directly
		for _, p := range fullPlugins {
			m.plugins[p.Name] = p
		}

		// If it was an index.json, fetch individual rule files concurrently
		if len(indexNames) > 0 {
			if sourceBaseURL == "" {
				sourceBaseURL = "https://raw.githubusercontent.com/Predidit/KazumiRules/main/"
			}
			if !strings.HasSuffix(sourceBaseURL, "/") {
				sourceBaseURL += "/"
			}

			client := &http.Client{Timeout: 10 * time.Second}
			var wg sync.WaitGroup
			var mu sync.Mutex
			sem := make(chan struct{}, 6) // max 6 concurrent downloads

			for _, name := range indexNames {
				wg.Add(1)
				go func(ruleName string) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()

					ruleUrl := sourceBaseURL + url.PathEscape(ruleName) + ".json"
					req, err := http.NewRequest(http.MethodGet, ruleUrl, nil)
					if err != nil {
						return
					}
					req.Header.Set("User-Agent", "akari-bridge/1.0")

					resp, err := client.Do(req)
					if err != nil || resp.StatusCode != http.StatusOK {
						return
					}
					defer resp.Body.Close()

					body, err := io.ReadAll(resp.Body)
					if err != nil {
						return
					}

					var plugin engine.Plugin
					if err := json.Unmarshal(body, &plugin); err == nil && plugin.Name != "" {
						if plugin.ID == "" {
							plugin.ID = "rule_" + strings.ToLower(plugin.Name)
						}
						plugin.Enabled = true
						mu.Lock()
						m.plugins[plugin.Name] = &plugin
						importedCount++
						mu.Unlock()
					}
				}(name)
			}
			wg.Wait()
		}

		m.saveRulesToFileLocked()
		return importedCount, nil
	}

	return 0, fmt.Errorf("invalid Kazumi plugin JSON format")
}

func (m *RuleManager) ImportPluginsFromURL(remoteUrl string) (int, error) {
	remoteUrl = strings.TrimSpace(remoteUrl)
	urlsToTry := []string{remoteUrl}

	// Auto generate mirror fallbacks for GitHub URLs
	if strings.Contains(remoteUrl, "raw.githubusercontent.com/Predidit/KazumiRules/") {
		branchPath := strings.TrimPrefix(remoteUrl, "https://raw.githubusercontent.com/Predidit/KazumiRules/")
		urlsToTry = append(urlsToTry,
			"https://fastly.jsdelivr.net/gh/Predidit/KazumiRules@"+branchPath,
			"https://ghproxy.net/"+remoteUrl,
		)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	var lastErr error
	var body []byte
	var successfulUrl string

	for _, tryUrl := range urlsToTry {
		req, err := http.NewRequest(http.MethodGet, tryUrl, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "akari-bridge/1.0")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode == http.StatusOK {
			b, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err == nil && len(b) > 0 {
				body = b
				successfulUrl = tryUrl
				break
			}
		} else {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP status %d from %s", resp.StatusCode, tryUrl)
		}
	}

	if len(body) == 0 {
		return 0, fmt.Errorf("failed to fetch from url: %v", lastErr)
	}

	// Compute base URL for sub-resource fetching
	base := successfulUrl
	if idx := strings.LastIndex(base, "/"); idx != -1 {
		base = base[:idx+1]
	}

	return m.ImportPluginsJSON(body, base)
}

func (m *RuleManager) saveRulesToFileLocked() {
	pluginFile := filepath.Join(m.cfg.DataDir, "plugins.json")
	list := make([]*engine.Plugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		list = append(list, p)
	}
	if data, err := json.MarshalIndent(list, "", "  "); err == nil {
		_ = os.WriteFile(pluginFile, data, 0644)
	}
}

func (m *RuleManager) FindPluginByUrl(pageUrl string) *engine.Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	u, err := url.Parse(pageUrl)
	if err != nil {
		return nil
	}
	host := strings.ToLower(u.Host)

	for _, p := range m.plugins {
		if bu, err := url.Parse(p.BaseURL); err == nil {
			if strings.EqualFold(bu.Host, host) {
				return p
			}
		}
	}
	return nil
}

