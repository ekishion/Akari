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
	stats, err := mgr.ImportPluginsJSON([]byte(jsonPayload), "")
	if err != nil {
		t.Fatalf("ImportPluginsJSON failed: %v", err)
	}
	if stats.TotalCount != 2 || stats.AddedCount != 2 {
		t.Errorf("expected 2 added plugins, got %+v", stats)
	}

	// 5. Delete plugin by Name
	if err := mgr.DeletePlugin("TestAnime"); err != nil {
		t.Fatalf("DeletePlugin failed: %v", err)
	}
	if _, ok := mgr.GetPluginByName("TestAnime"); ok {
		t.Errorf("expected TestAnime to be deleted")
	}

	// 6. Delete plugin by ID / lowercase / rule_ prefix
	if err := mgr.DeletePlugin("rule_imported1"); err != nil {
		t.Fatalf("DeletePlugin by ID failed: %v", err)
	}
	if _, ok := mgr.GetPluginByName("Imported1"); ok {
		t.Errorf("expected Imported1 to be deleted")
	}

	// 7. Delete plugin by URL encoded / case insensitive
	if err := mgr.DeletePlugin("imported2"); err != nil {
		t.Fatalf("DeletePlugin by lowercase name failed: %v", err)
	}
	if _, ok := mgr.GetPluginByName("Imported2"); ok {
		t.Errorf("expected Imported2 to be deleted")
	}
}

func TestRuleManager_DuplicateOverwrite(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataDir: tempDir,
	}

	mgr := NewRuleManager(cfg)

	// 1. Save rule v1.0 and disable it
	r1 := &engine.Plugin{
		ID:         "rule_sorani",
		Name:       "sorani",
		Version:    "1.0",
		BaseURL:    "https://old.sorani.net",
		SearchMode: "xpath",
		Enabled:    false,
	}
	_ = mgr.SavePlugin(r1)

	// 2. Import updated rule v2.0 with same ID
	updateJson := `[
		{
			"id": "rule_sorani",
			"name": "sorani",
			"version": "2.0",
			"baseURL": "https://new.sorani.net",
			"searchMode": "api"
		}
	]`
	stats, err := mgr.ImportPluginsJSON([]byte(updateJson), "")
	if err != nil {
		t.Fatalf("ImportPluginsJSON failed: %v", err)
	}
	if stats.UpdatedCount != 1 || stats.AddedCount != 0 {
		t.Errorf("expected 1 updated and 0 added, got %+v", stats)
	}

	// 3. Verify rule is updated in place, no duplicate, and enabled state preserved
	all := mgr.GetAllPlugins()
	if len(all) != 1 {
		t.Fatalf("expected exactly 1 plugin, got %d", len(all))
	}
	p := all[0]
	if p.Version != "2.0" || p.BaseURL != "https://new.sorani.net" || p.SearchMode != "api" {
		t.Errorf("plugin attributes not updated properly: %+v", p)
	}
	if p.Enabled != false {
		t.Errorf("expected user disabled state to be preserved, got enabled=%v", p.Enabled)
	}
}
