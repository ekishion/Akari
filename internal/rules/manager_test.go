package rules

import (
	"testing"

	"akari-bridge/internal/config"
	"akari-bridge/internal/engine"
)

func TestRuleManager_CRUD(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataDir: tempDir,
	}

	mgr := NewRuleManager(cfg)

	// 1. Check initial rules (starts empty, no built-in rules)
	initialPlugins := mgr.GetAllPlugins()
	if len(initialPlugins) != 0 {
		t.Fatalf("expected 0 initial plugins on clean setup, got %d", len(initialPlugins))
	}

	// 2. Add custom plugin
	custom := &engine.Plugin{
		ID:         "test_plugin",
		Name:       "TestAnime",
		Version:    "1.0",
		BaseURL:    "https://test.anime.com",
		SearchMode: "xpath",
		Enabled:    true,
	}
	if err := mgr.SavePlugin(custom); err != nil {
		t.Fatalf("SavePlugin failed: %v", err)
	}

	found, ok := mgr.GetPluginByName("TestAnime")
	if !ok || found == nil {
		t.Fatalf("expected to find TestAnime")
	}
	if found.BaseURL != "https://test.anime.com" {
		t.Errorf("unexpected BaseURL: %s", found.BaseURL)
	}

	// 3. Toggle plugin
	if err := mgr.TogglePlugin("TestAnime", false); err != nil {
		t.Fatalf("TogglePlugin failed: %v", err)
	}
	found, _ = mgr.GetPluginByName("TestAnime")
	if found.Enabled {
		t.Errorf("expected plugin to be disabled")
	}

	// 4. Import JSON Array
	jsonPayload := `[
		{
			"name": "Imported1",
			"version": "1.0",
			"baseURL": "https://imp1.com",
			"searchMode": "xpath",
			"enabled": true
		},
		{
			"name": "Imported2",
			"version": "2.0",
			"baseURL": "https://imp2.com",
			"searchMode": "api",
			"enabled": true
		}
	]`
	count, err := mgr.ImportPluginsJSON([]byte(jsonPayload), "")
	if err != nil {
		t.Fatalf("ImportPluginsJSON failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 imported plugins, got %d", count)
	}

	// 5. Delete plugin
	if err := mgr.DeletePlugin("TestAnime"); err != nil {
		t.Fatalf("DeletePlugin failed: %v", err)
	}
	if _, ok := mgr.GetPluginByName("TestAnime"); ok {
		t.Errorf("expected TestAnime to be deleted")
	}
}
