package storage

import (
	"testing"
	"time"

	"akari-bridge/internal/config"
)

func TestDB_Operations(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataDir: tempDir,
	}

	db, err := OpenDB(cfg)
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	defer db.Close()

	// 1. User operations
	now := time.Now()
	u := &UserRecord{
		ID:             "alice",
		Name:           "Alice",
		PasswordHash:   "secret",
		IsAdmin:        false,
		BgmAccessToken: "bgm_token_123",
		BgmUserID:      "alice_bgm",
		CreatedAt:      now,
	}
	if err := db.UpsertUser(u); err != nil {
		t.Fatalf("UpsertUser failed: %v", err)
	}

	fetched, err := db.GetUserByID("alice")
	if err != nil || fetched == nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if fetched.Name != "Alice" || fetched.BgmAccessToken != "bgm_token_123" {
		t.Errorf("unexpected user record: %+v", fetched)
	}

	// 2. Token operations
	if err := db.SaveToken("token_xyz", "alice", "Infuse AppleTV", "device1", nil); err != nil {
		t.Fatalf("SaveToken failed: %v", err)
	}

	userId, err := db.GetUserIDByToken("token_xyz")
	if err != nil || userId != "alice" {
		t.Errorf("expected userId alice, got %s (err: %v)", userId, err)
	}

	tokens, err := db.GetTokensByUserID("alice")
	if err != nil || len(tokens) != 1 {
		t.Errorf("expected 1 token for alice, got %d", len(tokens))
	}

	// 3. Playback progress & history
	if err := db.UpdatePlaybackProgress("alice", "bgm_ep_100", 500000000, 1000000000); err != nil {
		t.Fatalf("UpdatePlaybackProgress failed: %v", err)
	}

	itemData := db.GetUserItemData("alice", "bgm_ep_100")
	if itemData == nil || itemData.PlaybackPositionTicks != 500000000 {
		t.Errorf("unexpected itemData: %+v", itemData)
	}

	resumeList := db.GetUserResumeItems("alice", 10)
	if len(resumeList) != 1 || resumeList[0] != "bgm_ep_100" {
		t.Errorf("unexpected resume list: %+v", resumeList)
	}

	// Mark as played
	if err := db.MarkPlayed("alice", "bgm_ep_100", true); err != nil {
		t.Fatalf("MarkPlayed failed: %v", err)
	}
	itemDataAfter := db.GetUserItemData("alice", "bgm_ep_100")
	if !itemDataAfter.Played {
		t.Errorf("expected item to be marked played")
	}

	// 4. Stats
	stats, err := db.GetStats()
	if err != nil || stats == nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if stats.UserCount < 1 || stats.TokenCount < 1 || stats.HistoryCount < 1 {
		t.Errorf("unexpected stats: %+v", stats)
	}
}

func TestDB_FavoritePersistenceOnRestart(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataDir: tempDir,
	}

	// 1. First run: open DB, mark favorite
	db1, err := OpenDB(cfg)
	if err != nil {
		t.Fatalf("First OpenDB failed: %v", err)
	}

	if err := db1.MarkFavorite("admin", "bgm_sub_622288", true); err != nil {
		t.Fatalf("MarkFavorite failed: %v", err)
	}
	if err := db1.MarkFavorite("admin", "bgm_ep_622288_1_1741638", true); err != nil {
		t.Fatalf("MarkFavorite ep failed: %v", err)
	}

	favs1 := db1.GetUserFavoriteItemIDs("admin", 10)
	if len(favs1) != 2 {
		t.Fatalf("expected 2 favorites before restart, got %d", len(favs1))
	}

	_ = db1.Close()

	// 2. Second run: simulate restart by reopening DB
	db2, err := OpenDB(cfg)
	if err != nil {
		t.Fatalf("Second OpenDB failed: %v", err)
	}
	defer db2.Close()

	favs2 := db2.GetUserFavoriteItemIDs("admin", 10)
	if len(favs2) != 2 {
		t.Fatalf("expected 2 favorites to persist after restart, got %d", len(favs2))
	}

	itemData := db2.GetUserItemData("admin", "bgm_sub_622288")
	if !itemData.IsFavorite {
		t.Errorf("expected bgm_sub_622288 IsFavorite to be true after restart")
	}

	// 3. Test watching subject IDs only returns favorited subjects
	watchingSubs := db2.GetUserWatchingSubjectIDs("admin", 10)
	if len(watchingSubs) != 1 || watchingSubs[0] != 622288 {
		t.Errorf("expected watching subject 622288, got %v", watchingSubs)
	}

	// 4. Test unmarking all subject favorites
	if err := db2.UnmarkAllSubjectFavorites("admin", 622288); err != nil {
		t.Fatalf("UnmarkAllSubjectFavorites failed: %v", err)
	}
	watchingSubsAfter := db2.GetUserWatchingSubjectIDs("admin", 10)
	if len(watchingSubsAfter) != 0 {
		t.Errorf("expected 0 watching subjects after unmark, got %v", watchingSubsAfter)
	}
}

func TestDB_SettingsPersistence(t *testing.T) {
	tempDir := t.TempDir()
	cfg1 := &config.Config{
		DataDir:     tempDir,
		BangumiHost: "https://api.bgm.tv",
		DanDanHost:  "https://api.dandanplay.net",
		ServerName:  "Akari Default",
	}

	db1, err := OpenDB(cfg1)
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}

	if err := db1.SaveSetting("bangumi_host", "https://mirror.bgm.rin.cat"); err != nil {
		t.Fatalf("SaveSetting failed: %v", err)
	}
	if err := db1.SaveSetting("dandan_host", "https://ddplay.retr0.xyz"); err != nil {
		t.Fatalf("SaveSetting failed: %v", err)
	}
	if err := db1.SaveSetting("server_name", "My Custom Akari"); err != nil {
		t.Fatalf("SaveSetting failed: %v", err)
	}

	val, err := db1.GetSetting("bangumi_host")
	if err != nil || val != "https://mirror.bgm.rin.cat" {
		t.Fatalf("unexpected GetSetting bangumi_host: %s, err: %v", val, err)
	}

	all, err := db1.GetAllSettings()
	if err != nil || len(all) < 3 {
		t.Fatalf("unexpected GetAllSettings: %+v", all)
	}

	_ = db1.Close()

	// Reopen with defaults, should load persisted settings automatically
	cfg2 := &config.Config{
		DataDir:     tempDir,
		BangumiHost: "https://api.bgm.tv",
		DanDanHost:  "https://api.dandanplay.net",
		ServerName:  "Akari Default",
	}
	db2, err := OpenDB(cfg2)
	if err != nil {
		t.Fatalf("Second OpenDB failed: %v", err)
	}
	defer db2.Close()

	if cfg2.BangumiHost != "https://mirror.bgm.rin.cat" {
		t.Errorf("expected BangumiHost to persist, got %s", cfg2.BangumiHost)
	}
	if cfg2.DanDanHost != "https://ddplay.retr0.xyz" {
		t.Errorf("expected DanDanHost to persist, got %s", cfg2.DanDanHost)
	}
	if cfg2.ServerName != "My Custom Akari" {
		t.Errorf("expected ServerName to persist, got %s", cfg2.ServerName)
	}
}

