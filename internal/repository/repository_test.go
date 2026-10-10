package repository

import (
	"os"
	"testing"
	"time"

	"akari-bridge/internal/config"
	"akari-bridge/internal/domain"
	"akari-bridge/internal/engine"
)

func setupTestDB(t *testing.T) (*DBConn, func()) {
	tmpDir, err := os.MkdirTemp("", "repo_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cfg := &config.Config{
		DataDir: tmpDir,
	}

	conn, err := OpenDB(cfg)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to open test db: %v", err)
	}

	cleanup := func() {
		_ = conn.Close()
		_ = os.RemoveAll(tmpDir)
	}

	return conn, cleanup
}

func TestUserRepository(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewUserRepository(conn)

	// Create user
	u := &domain.User{
		ID:        "user1",
		Name:      "Alice",
		IsAdmin:   true,
		CreatedAt: time.Now(),
	}
	err := repo.CreateUser(u, "secret123")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Get user
	got, err := repo.GetUser("user1")
	if err != nil || got == nil || got.Name != "Alice" {
		t.Fatalf("expected user Alice, got %v (err: %v)", got, err)
	}

	// Easy PIN
	if err := repo.SaveUserEasyPassword("user1", "1234"); err != nil {
		t.Fatalf("failed to save easy pin: %v", err)
	}
	if !repo.VerifyUserEasyPassword("user1", "1234") {
		t.Fatalf("expected easy pin verification to pass")
	}
	if repo.VerifyUserEasyPassword("user1", "9999") {
		t.Fatalf("expected wrong easy pin to fail")
	}

	// Custom token
	if err := repo.SaveUserCustomToken("user1", "custom_token_abc"); err != nil {
		t.Fatalf("failed to save custom token: %v", err)
	}
	tokUser, err := repo.GetUserByCustomToken("custom_token_abc")
	if err != nil || tokUser == nil || tokUser.ID != "user1" {
		t.Fatalf("expected user1 from token lookup, got %v (err: %v)", tokUser, err)
	}
}

func TestPlaybackAndFavoriteRepository(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	playbackRepo := NewPlaybackRepository(conn)
	favRepo := NewFavoriteRepository(conn)

	// Duration & Progress
	_ = playbackRepo.SaveItemDuration("item1", 10000000)
	if dur := playbackRepo.GetItemTotalTicks("item1"); dur != 10000000 {
		t.Fatalf("expected duration 10000000, got %d", dur)
	}

	_ = playbackRepo.UpdatePlaybackProgress("admin", "item1", 5000000, 10000000)
	pos, tot, played := playbackRepo.GetPlaybackProgress("admin", "item1")
	if pos != 5000000 || tot != 10000000 || played {
		t.Fatalf("unexpected playback progress: pos=%d, tot=%d, played=%v", pos, tot, played)
	}

	// Favorite
	if favRepo.IsFavoriteItem("admin", "item1") {
		t.Fatalf("item1 should not be favorite yet")
	}
	_ = favRepo.ToggleFavoriteItem("admin", "item1", true)
	if !favRepo.IsFavoriteItem("admin", "item1") {
		t.Fatalf("item1 should be favorite now")
	}
	favs, _ := favRepo.ListFavoriteItems("admin")
	if len(favs) != 1 || favs[0] != "item1" {
		t.Fatalf("expected [item1] favorites, got %v", favs)
	}
}

func TestRuleAndSynonymRepository(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	ruleRepo := NewRuleRepository(conn)
	synRepo := NewSynonymRepository(conn)

	// Rule Plugin
	p := &engine.Plugin{
		Name:    "TestRule",
		Version: "1.0",
		Enabled: true,
	}
	if err := ruleRepo.SavePlugin(p); err != nil {
		t.Fatalf("failed to save plugin: %v", err)
	}
	gotP, err := ruleRepo.GetPluginByName("TestRule")
	if err != nil || gotP == nil || gotP.Name != "TestRule" {
		t.Fatalf("expected TestRule plugin, got %v", gotP)
	}

	// Synonym
	_ = synRepo.SaveGlobalSynonyms(map[string]string{
		"超": "超时空",
	})
	syns, _ := synRepo.GetGlobalSynonyms()
	if syns["超"] != "超时空" {
		t.Fatalf("expected synonym '超' -> '超时空', got %v", syns)
	}

	// Aliases
	_ = synRepo.SaveSubjectAliases(12345, []string{"Alias1", "Alias2"})
	aliases, _ := synRepo.GetSubjectAliases(12345)
	if len(aliases) != 2 || aliases[0] != "Alias1" {
		t.Fatalf("expected aliases [Alias1, Alias2], got %v", aliases)
	}
}
