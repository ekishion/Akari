package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
	"akari-bridge/internal/config"
	"akari-bridge/internal/model"
)

type DB struct {
	db  *sql.DB
	cfg *config.Config
	mu  sync.RWMutex
}

type PlaybackRecord struct {
	ItemId         string    `json:"itemId"`
	PositionTicks  int64     `json:"positionTicks"`
	TotalTicks     int64     `json:"totalTicks"`
	Played         bool      `json:"played"`
	PlayCount      int       `json:"playCount"`
	LastPlayedDate time.Time `json:"lastPlayedDate"`
}

type UserRecord struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	PasswordHash   string     `json:"passwordHash"`
	IsAdmin        bool       `json:"isAdmin"`
	IsDisabled     bool       `json:"isDisabled"`
	BgmAccessToken string     `json:"bgmAccessToken"`
	BgmUserID      string     `json:"bgmUserId"`
	LastLoginAt    *time.Time `json:"lastLoginAt"`
	CreatedAt      time.Time  `json:"createdAt"`
}

func OpenDB(cfg *config.Config) (*DB, error) {
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}

	dbPath := filepath.Join(cfg.DataDir, "bridge.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	db.SetMaxOpenConns(1) // SQLite works best with 1 writer / serialized in embedded mode
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	s := &DB{
		db:  db,
		cfg: cfg,
	}

	if err := s.initSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to init db schema: %w", err)
	}

	s.migrateFromJSON()

	// Load persisted system settings
	if bgmHost, err := s.GetSetting("bangumi_host"); err == nil && bgmHost != "" {
		cfg.BangumiHost = bgmHost
	}
	if danHost, err := s.GetSetting("dandan_host"); err == nil && danHost != "" {
		cfg.DanDanHost = danHost
	}
	if srvName, err := s.GetSetting("server_name"); err == nil && srvName != "" {
		cfg.ServerName = srvName
	}

	log.Printf("[DB] SQLite database initialized at %s", dbPath)
	return s, nil
}

func (s *DB) Close() error {
	return s.db.Close()
}

func (s *DB) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		password_hash TEXT DEFAULT '',
		is_admin INTEGER DEFAULT 0,
		is_disabled INTEGER DEFAULT 0,
		bgm_access_token TEXT DEFAULT '',
		bgm_user_id TEXT DEFAULT '',
		last_login_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS user_tokens (
		token TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		client_name TEXT DEFAULT '',
		device_id TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		expires_at DATETIME,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS playback_histories (
		user_id TEXT NOT NULL,
		item_id TEXT NOT NULL,
		position_ticks INTEGER DEFAULT 0,
		total_ticks INTEGER DEFAULT 0,
		played INTEGER DEFAULT 0,
		play_count INTEGER DEFAULT 0,
		is_favorite INTEGER DEFAULT 0,
		last_played_date DATETIME,
		PRIMARY KEY (user_id, item_id)
	);

	CREATE TABLE IF NOT EXISTS plugins (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		version TEXT DEFAULT '',
		enabled INTEGER DEFAULT 1,
		data_json TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS system_settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS global_synonyms (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		pattern TEXT NOT NULL UNIQUE,
		replacement TEXT NOT NULL,
		enabled INTEGER DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS subject_aliases (
		subject_id INTEGER PRIMARY KEY,
		title TEXT DEFAULT '',
		aliases_json TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS admin_account (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS audit_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		event_type TEXT NOT NULL,
		ip_address TEXT DEFAULT '',
		user_agent TEXT DEFAULT '',
		details TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS ip_bans (
		ip TEXT PRIMARY KEY,
		reason TEXT DEFAULT '',
		banned_until DATETIME,
		failed_attempts INTEGER DEFAULT 0,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_tokens_user ON user_tokens(user_id);
	CREATE INDEX IF NOT EXISTS idx_histories_user ON playback_histories(user_id);
	CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_logs(created_at DESC);
	`

	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	// Seed default global synonyms if empty
	s.seedDefaultSynonyms()

	// Try adding is_favorite column if upgrading from older schema
	_, _ = s.db.Exec(`ALTER TABLE playback_histories ADD COLUMN is_favorite INTEGER DEFAULT 0`)
	return nil
}

func (s *DB) seedDefaultSynonyms() {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM global_synonyms`).Scan(&count); err == nil && count == 0 {
		defaultSeeds := [][2]string{
			{"超时空", "超"},
			{"已经死了", "已死"},
			{"电影版", "剧场版"},
			{"总集篇", "剧场版"},
		}
		for _, pair := range defaultSeeds {
			_, _ = s.db.Exec(`INSERT OR IGNORE INTO global_synonyms (pattern, replacement, enabled) VALUES (?, ?, 1)`, pair[0], pair[1])
		}
	}
}

func (s *DB) migrateFromJSON() {
	// 1. Migrate playback history if exists
	histFile := filepath.Join(s.cfg.DataDir, "playback_history.json")
	if data, err := os.ReadFile(histFile); err == nil {
		var records map[string]PlaybackRecord
		if err := json.Unmarshal(data, &records); err == nil && len(records) > 0 {
			tx, err := s.db.Begin()
			if err == nil {
				stmt, err := tx.Prepare(`
					INSERT OR IGNORE INTO playback_histories 
					(user_id, item_id, position_ticks, total_ticks, played, play_count, is_favorite, last_played_date)
					VALUES (?, ?, ?, ?, ?, ?, 0, ?)
				`)
				if err == nil {
					for k, r := range records {
						userId := "admin"
						itemId := r.ItemId
						// Key format is userId:itemId or legacy 00000000000000000000000000000001:itemId
						if idx := len(k); idx > 0 {
							for i := 0; i < len(k); i++ {
								if k[i] == ':' {
									userId = k[:i]
									itemId = k[i+1:]
									break
								}
							}
						}
						if userId == "00000000000000000000000000000001" || userId == "" || userId == "default" {
							userId = "admin"
						}
						playedInt := 0
						if r.Played {
							playedInt = 1
						}
						_, _ = stmt.Exec(userId, itemId, r.PositionTicks, r.TotalTicks, playedInt, r.PlayCount, r.LastPlayedDate)
					}
					_ = stmt.Close()
					_ = tx.Commit()
					log.Printf("[DB] Migrated %d playback records from playback_history.json into SQLite", len(records))
				}
			}
		}
		// Rename migrated file to .bak to avoid re-reading and overwriting DB state on future restarts
		_ = os.Rename(histFile, histFile+".bak")
	}
}

// User Operations

func (s *DB) UpsertUser(u *UserRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	isAdminInt := 0
	if u.IsAdmin {
		isAdminInt = 1
	}
	isDisabledInt := 0
	if u.IsDisabled {
		isDisabledInt = 1
	}

	query := `
	INSERT INTO users (id, name, password_hash, is_admin, is_disabled, bgm_access_token, bgm_user_id, last_login_at, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		name = excluded.name,
		password_hash = excluded.password_hash,
		is_admin = excluded.is_admin,
		is_disabled = excluded.is_disabled,
		bgm_access_token = excluded.bgm_access_token,
		bgm_user_id = excluded.bgm_user_id,
		last_login_at = excluded.last_login_at
	`
	_, err := s.db.Exec(query, u.ID, u.Name, u.PasswordHash, isAdminInt, isDisabledInt, u.BgmAccessToken, u.BgmUserID, u.LastLoginAt, u.CreatedAt)
	return err
}

func (s *DB) GetUserByID(id string) (*UserRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if id == "00000000000000000000000000000001" || id == "" {
		id = "admin"
	}

	row := s.db.QueryRow(`
		SELECT id, name, password_hash, is_admin, is_disabled, bgm_access_token, bgm_user_id, last_login_at, created_at
		FROM users WHERE id = ? OR name = ?
	`, id, id)

	var u UserRecord
	var isAdminInt, isDisabledInt int
	var lastLoginAt sql.NullTime

	err := row.Scan(&u.ID, &u.Name, &u.PasswordHash, &isAdminInt, &isDisabledInt, &u.BgmAccessToken, &u.BgmUserID, &lastLoginAt, &u.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	u.IsAdmin = isAdminInt == 1
	u.IsDisabled = isDisabledInt == 1
	if lastLoginAt.Valid {
		u.LastLoginAt = &lastLoginAt.Time
	}
	return &u, nil
}

func (s *DB) ListUsers() ([]*UserRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, name, password_hash, is_admin, is_disabled, bgm_access_token, bgm_user_id, last_login_at, created_at
		FROM users ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*UserRecord
	for rows.Next() {
		var u UserRecord
		var isAdminInt, isDisabledInt int
		var lastLoginAt sql.NullTime

		if err := rows.Scan(&u.ID, &u.Name, &u.PasswordHash, &isAdminInt, &isDisabledInt, &u.BgmAccessToken, &u.BgmUserID, &lastLoginAt, &u.CreatedAt); err != nil {
			continue
		}
		u.IsAdmin = isAdminInt == 1
		u.IsDisabled = isDisabledInt == 1
		if lastLoginAt.Valid {
			u.LastLoginAt = &lastLoginAt.Time
		}
		list = append(list, &u)
	}
	return list, nil
}

func (s *DB) DeleteUser(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}

// Token Operations

func (s *DB) SaveToken(token, userId, clientName, deviceId string, expiresAt *time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO user_tokens (token, user_id, client_name, device_id, expires_at)
		VALUES (?, ?, ?, ?, ?)
	`, token, userId, clientName, deviceId, expiresAt)
	return err
}

func (s *DB) GetUserIDByToken(token string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var userId string
	var expiresAt sql.NullTime

	err := s.db.QueryRow(`SELECT user_id, expires_at FROM user_tokens WHERE token = ?`, token).Scan(&userId, &expiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}

	if expiresAt.Valid && expiresAt.Time.Before(time.Now()) {
		return "", nil
	}
	return userId, nil
}

func (s *DB) RevokeToken(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM user_tokens WHERE token = ?`, token)
	return err
}

func (s *DB) RevokeAllUserTokens(userId string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM user_tokens WHERE user_id = ?`, userId)
	return err
}

// Playback History Operations

func (s *DB) UpdatePlaybackProgress(userId, itemId string, ticks, totalTicks int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userId == "00000000000000000000000000000001" || userId == "" || userId == "default" {
		userId = "admin"
	}

	played := 0
	if totalTicks > 0 && float64(ticks)/float64(totalTicks) >= 0.90 {
		played = 1
	}

	query := `
	INSERT INTO playback_histories (user_id, item_id, position_ticks, total_ticks, played, last_played_date)
	VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(user_id, item_id) DO UPDATE SET
		position_ticks = excluded.position_ticks,
		total_ticks = CASE WHEN excluded.total_ticks > 0 THEN excluded.total_ticks ELSE playback_histories.total_ticks END,
		played = CASE WHEN excluded.played = 1 THEN 1 ELSE playback_histories.played END,
		last_played_date = excluded.last_played_date
	`
	_, err := s.db.Exec(query, userId, itemId, ticks, totalTicks, played, time.Now())
	return err
}

func (s *DB) MarkPlayed(userId, itemId string, played bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userId == "00000000000000000000000000000001" || userId == "" || userId == "default" {
		userId = "admin"
	}

	playedInt := 0
	if played {
		playedInt = 1
	}

	query := `
	INSERT INTO playback_histories (user_id, item_id, position_ticks, total_ticks, played, play_count, last_played_date)
	VALUES (?, ?, CASE WHEN ? = 1 THEN 10000 ELSE 0 END, 10000, ?, ?, ?)
	ON CONFLICT(user_id, item_id) DO UPDATE SET
		played = excluded.played,
		play_count = play_count + CASE WHEN excluded.played = 1 THEN 1 ELSE 0 END,
		position_ticks = CASE WHEN excluded.played = 1 THEN total_ticks ELSE 0 END,
		last_played_date = excluded.last_played_date
	`
	_, err := s.db.Exec(query, userId, itemId, playedInt, playedInt, playedInt, time.Now())
	return err
}

func (s *DB) MarkFavorite(userId, itemId string, isFavorite bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userId == "00000000000000000000000000000001" || userId == "" || userId == "default" {
		userId = "admin"
	}

	favInt := 0
	if isFavorite {
		favInt = 1
	}

	query := `
	INSERT INTO playback_histories (user_id, item_id, is_favorite, last_played_date)
	VALUES (?, ?, ?, ?)
	ON CONFLICT(user_id, item_id) DO UPDATE SET
		is_favorite = excluded.is_favorite,
		last_played_date = excluded.last_played_date
	`
	_, err := s.db.Exec(query, userId, itemId, favInt, time.Now())
	return err
}

func (s *DB) SaveItemDuration(itemId string, totalTicks int64) error {
	if totalTicks <= 0 || itemId == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	INSERT INTO playback_histories (user_id, item_id, total_ticks, last_played_date)
	VALUES ('admin', ?, ?, ?)
	ON CONFLICT(user_id, item_id) DO UPDATE SET
		total_ticks = CASE WHEN excluded.total_ticks > 0 THEN excluded.total_ticks ELSE playback_histories.total_ticks END
	`
	_, err := s.db.Exec(query, itemId, totalTicks, time.Now())
	return err
}

func (s *DB) GetItemTotalTicks(itemId string) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var ticks int64
	_ = s.db.QueryRow(`
		SELECT total_ticks FROM playback_histories
		WHERE item_id = ? AND total_ticks > 0
		ORDER BY total_ticks DESC LIMIT 1
	`, itemId).Scan(&ticks)
	return ticks
}

func (s *DB) GetUserItemData(userId, itemId string) *model.UserItemDataDto {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if userId == "00000000000000000000000000000001" || userId == "" || userId == "default" {
		userId = "admin"
	}

	var posTicks, totalTicks int64
	var played, playCount, isFavorite int
	var lastPlayed sql.NullTime

	err := s.db.QueryRow(`
		SELECT position_ticks, total_ticks, played, play_count, is_favorite, last_played_date
		FROM playback_histories WHERE user_id = ? AND item_id = ?
	`, userId, itemId).Scan(&posTicks, &totalTicks, &played, &playCount, &isFavorite, &lastPlayed)

	if err != nil {
		return &model.UserItemDataDto{
			Played:                false,
			PlaybackPositionTicks: 0,
			PlayCount:            0,
			IsFavorite:           false,
			Key:                  itemId,
		}
	}

	lastPlayedStr := ""
	if lastPlayed.Valid {
		lastPlayedStr = lastPlayed.Time.UTC().Format(time.RFC3339)
	}

	playedPct := 0.0
	if totalTicks > 0 {
		playedPct = float64(posTicks) / float64(totalTicks) * 100.0
	}

	return &model.UserItemDataDto{
		Played:                played == 1,
		PlaybackPositionTicks: posTicks,
		PlayCount:            playCount,
		PlayedPercentage:     playedPct,
		IsFavorite:           isFavorite == 1,
		LastPlayedDate:        lastPlayedStr,
		Key:                  itemId,
	}
}

func (s *DB) GetUserFavoriteItemIDs(userId string, limit int) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if userId == "00000000000000000000000000000001" || userId == "" || userId == "default" {
		userId = "admin"
	}
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.Query(`
		SELECT item_id FROM playback_histories
		WHERE user_id = ? AND is_favorite = 1
		ORDER BY last_played_date DESC LIMIT ?
	`, userId, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var itemIds []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			itemIds = append(itemIds, id)
		}
	}
	return itemIds
}

// GetUserWatchingSubjectIDs returns unique Bangumi subject IDs that the user has favorited
func (s *DB) GetUserWatchingSubjectIDs(userId string, limit int) []int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if userId == "00000000000000000000000000000001" || userId == "" || userId == "default" {
		userId = "admin"
	}
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.Query(`
		SELECT item_id FROM playback_histories
		WHERE user_id = ? AND is_favorite = 1
		ORDER BY last_played_date DESC LIMIT 100
	`, userId)
	if err != nil {
		return nil
	}
	defer rows.Close()

	seen := make(map[int]bool)
	var subIds []int

	for rows.Next() {
		var itemId string
		if err := rows.Scan(&itemId); err == nil {
			sid := 0
			if strings.HasPrefix(itemId, "bgm_sub_") {
				sid, _ = strconv.Atoi(strings.TrimPrefix(itemId, "bgm_sub_"))
			} else if strings.HasPrefix(itemId, "bgm_ep_") {
				clean := strings.TrimPrefix(itemId, "bgm_ep_")
				parts := strings.Split(clean, "_")
				if len(parts) > 0 {
					sid, _ = strconv.Atoi(parts[0])
				}
			} else if strings.HasPrefix(itemId, "bgm_season_") {
				clean := strings.TrimPrefix(itemId, "bgm_season_")
				parts := strings.Split(clean, "_")
				if len(parts) > 0 {
					sid, _ = strconv.Atoi(parts[0])
				}
			}
			if sid > 0 && !seen[sid] {
				seen[sid] = true
				subIds = append(subIds, sid)
				if len(subIds) >= limit {
					break
				}
			}
		}
	}
	return subIds
}

// UnmarkAllSubjectFavorites clears is_favorite for a subject and all its sub-items
func (s *DB) UnmarkAllSubjectFavorites(userId string, subjectId int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userId == "00000000000000000000000000000001" || userId == "" || userId == "default" {
		userId = "admin"
	}

	prefixSub := fmt.Sprintf("bgm_sub_%d", subjectId)
	prefixEp := fmt.Sprintf("bgm_ep_%d_%%", subjectId)
	prefixSeason := fmt.Sprintf("bgm_season_%d_%%", subjectId)

	_, err := s.db.Exec(`
		UPDATE playback_histories SET is_favorite = 0
		WHERE user_id = ? AND (item_id = ? OR item_id LIKE ? OR item_id LIKE ?)
	`, userId, prefixSub, prefixEp, prefixSeason)
	return err
}

func (s *DB) GetUserResumeItems(userId string, limit int) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if userId == "00000000000000000000000000000001" || userId == "" || userId == "default" {
		userId = "admin"
	}
	if limit <= 0 {
		limit = 20
	}

	rows, err := s.db.Query(`
		SELECT item_id FROM playback_histories
		WHERE user_id = ? AND played = 0 AND position_ticks > 0
		ORDER BY last_played_date DESC LIMIT ?
	`, userId, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var itemIds []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			itemIds = append(itemIds, id)
		}
	}
	return itemIds
}

type TokenRecord struct {
	Token      string     `json:"token"`
	UserID     string     `json:"userId"`
	ClientName string     `json:"clientName"`
	DeviceID   string     `json:"deviceId"`
	CreatedAt  time.Time  `json:"createdAt"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
}

func (s *DB) GetTokensByUserID(userId string) ([]*TokenRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if userId == "00000000000000000000000000000001" || userId == "" {
		userId = "admin"
	}

	rows, err := s.db.Query(`
		SELECT token, user_id, client_name, device_id, created_at, expires_at
		FROM user_tokens WHERE user_id = ? ORDER BY created_at DESC
	`, userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*TokenRecord
	for rows.Next() {
		var t TokenRecord
		var exp sql.NullTime
		if err := rows.Scan(&t.Token, &t.UserID, &t.ClientName, &t.DeviceID, &t.CreatedAt, &exp); err == nil {
			if exp.Valid {
				t.ExpiresAt = &exp.Time
			}
			list = append(list, &t)
		}
	}
	return list, nil
}

type PlaybackHistoryDetail struct {
	UserID         string     `json:"userId"`
	ItemID         string     `json:"itemId"`
	PositionTicks  int64      `json:"positionTicks"`
	TotalTicks     int64      `json:"totalTicks"`
	Played         bool       `json:"played"`
	PlayCount      int        `json:"playCount"`
	LastPlayedDate *time.Time `json:"lastPlayedDate"`
}

func (s *DB) ListRecentHistories(limit int) ([]*PlaybackHistoryDetail, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.Query(`
		SELECT user_id, item_id, position_ticks, total_ticks, played, play_count, last_played_date
		FROM playback_histories
		ORDER BY last_played_date DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*PlaybackHistoryDetail
	for rows.Next() {
		var h PlaybackHistoryDetail
		var playedInt int
		var lpd sql.NullTime
		if err := rows.Scan(&h.UserID, &h.ItemID, &h.PositionTicks, &h.TotalTicks, &playedInt, &h.PlayCount, &lpd); err == nil {
			h.Played = playedInt == 1
			if lpd.Valid {
				h.LastPlayedDate = &lpd.Time
			}
			list = append(list, &h)
		}
	}
	return list, nil
}

type SystemStats struct {
	UserCount      int   `json:"userCount"`
	TokenCount     int   `json:"tokenCount"`
	HistoryCount   int   `json:"historyCount"`
	DbSizeBytes    int64 `json:"dbSizeBytes"`
}

func (s *DB) GetStats() (*SystemStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &SystemStats{}

	_ = s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&stats.UserCount)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM user_tokens`).Scan(&stats.TokenCount)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM playback_histories`).Scan(&stats.HistoryCount)

	dbPath := filepath.Join(s.cfg.DataDir, "bridge.db")
	if fi, err := os.Stat(dbPath); err == nil {
		stats.DbSizeBytes = fi.Size()
	}

	return stats, nil
}

func (s *DB) GetSetting(key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var val string
	err := s.db.QueryRow(`SELECT value FROM system_settings WHERE key = ?`, key).Scan(&val)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return val, nil
}

func (s *DB) SaveSetting(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	INSERT INTO system_settings (key, value, updated_at)
	VALUES (?, ?, ?)
	ON CONFLICT(key) DO UPDATE SET
		value = excluded.value,
		updated_at = excluded.updated_at
	`
	_, err := s.db.Exec(query, key, value, time.Now())
	return err
}

func (s *DB) GetAllSettings() (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`SELECT key, value FROM system_settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err == nil {
			res[k] = v
		}
	}
	return res, nil
}

// Global Synonyms

type GlobalSynonymRecord struct {
	ID          int64     `json:"id"`
	Pattern     string    `json:"pattern"`
	Replacement string    `json:"replacement"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (s *DB) GetGlobalSynonyms() (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`SELECT pattern, replacement FROM global_synonyms WHERE enabled = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string]string)
	for rows.Next() {
		var p, r string
		if err := rows.Scan(&p, &r); err == nil {
			res[p] = r
		}
	}
	return res, nil
}

func (s *DB) ListGlobalSynonyms() ([]GlobalSynonymRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`SELECT id, pattern, replacement, enabled, created_at FROM global_synonyms ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []GlobalSynonymRecord
	for rows.Next() {
		var rec GlobalSynonymRecord
		var enInt int
		if err := rows.Scan(&rec.ID, &rec.Pattern, &rec.Replacement, &enInt, &rec.CreatedAt); err == nil {
			rec.Enabled = enInt == 1
			list = append(list, rec)
		}
	}
	return list, nil
}

func (s *DB) UpsertGlobalSynonym(pattern, replacement string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	enInt := 0
	if enabled {
		enInt = 1
	}

	query := `
	INSERT INTO global_synonyms (pattern, replacement, enabled)
	VALUES (?, ?, ?)
	ON CONFLICT(pattern) DO UPDATE SET
		replacement = excluded.replacement,
		enabled = excluded.enabled
	`
	_, err := s.db.Exec(query, strings.TrimSpace(pattern), strings.TrimSpace(replacement), enInt)
	return err
}

func (s *DB) DeleteGlobalSynonym(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM global_synonyms WHERE id = ?`, id)
	return err
}

func (s *DB) DeleteGlobalSynonymByPattern(pattern string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM global_synonyms WHERE pattern = ?`, pattern)
	return err
}

func (s *DB) ResetDefaultGlobalSynonyms() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.db.Exec(`DELETE FROM global_synonyms`); err != nil {
		return err
	}

	defaultSeeds := [][2]string{
		{"超时空", "超"},
		{"已经死了", "已死"},
		{"电影版", "剧场版"},
		{"总集篇", "剧场版"},
	}
	for _, pair := range defaultSeeds {
		_, _ = s.db.Exec(`INSERT INTO global_synonyms (pattern, replacement, enabled) VALUES (?, ?, 1)`, pair[0], pair[1])
	}
	return nil
}

// Subject Aliases

type SubjectAliasRecord struct {
	SubjectID int       `json:"subjectId"`
	Title     string    `json:"title"`
	Aliases   []string  `json:"aliases"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (s *DB) GetSubjectAliases(subjectId int) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var raw string
	err := s.db.QueryRow(`SELECT aliases_json FROM subject_aliases WHERE subject_id = ?`, subjectId).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	var aliases []string
	if err := json.Unmarshal([]byte(raw), &aliases); err != nil {
		return nil, err
	}
	return aliases, nil
}

func (s *DB) ListSubjectAliases() ([]SubjectAliasRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`SELECT subject_id, title, aliases_json, updated_at FROM subject_aliases ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []SubjectAliasRecord
	for rows.Next() {
		var rec SubjectAliasRecord
		var raw string
		if err := rows.Scan(&rec.SubjectID, &rec.Title, &raw, &rec.UpdatedAt); err == nil {
			_ = json.Unmarshal([]byte(raw), &rec.Aliases)
			list = append(list, rec)
		}
	}
	return list, nil
}

func (s *DB) UpsertSubjectAliases(subjectId int, title string, aliases []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(aliases)
	if err != nil {
		return err
	}

	query := `
	INSERT INTO subject_aliases (subject_id, title, aliases_json, updated_at)
	VALUES (?, ?, ?, ?)
	ON CONFLICT(subject_id) DO UPDATE SET
		title = excluded.title,
		aliases_json = excluded.aliases_json,
		updated_at = excluded.updated_at
	`
	_, err = s.db.Exec(query, subjectId, strings.TrimSpace(title), string(data), time.Now())
	return err
}

func (s *DB) DeleteSubjectAliases(subjectId int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM subject_aliases WHERE subject_id = ?`, subjectId)
	return err
}

// -------------------------------------------------------------
// Admin Account & Security Management
// -------------------------------------------------------------

type AdminAccount struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type AuditLogRecord struct {
	ID        int       `json:"id"`
	EventType string    `json:"eventType"` // "login_success", "login_failed", "rule_updated", "config_changed", "ip_banned", "ip_unbanned"
	IPAddress string    `json:"ipAddress"`
	UserAgent string    `json:"userAgent"`
	Details   string    `json:"details"`
	CreatedAt time.Time `json:"createdAt"`
}

type IPBanRecord struct {
	IP             string     `json:"ip"`
	Reason         string     `json:"reason"`
	BannedUntil    *time.Time `json:"bannedUntil"`
	FailedAttempts int        `json:"failedAttempts"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

func (s *DB) GetAdminAccount() (*AdminAccount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var acc AdminAccount
	err := s.db.QueryRow(`SELECT id, username, password_hash, created_at, updated_at FROM admin_account ORDER BY id ASC LIMIT 1`).
		Scan(&acc.ID, &acc.Username, &acc.PasswordHash, &acc.CreatedAt, &acc.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &acc, nil
}

func (s *DB) EnsureDefaultAdmin(username, passwordHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM admin_account`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		now := time.Now()
		_, err := s.db.Exec(`INSERT INTO admin_account (username, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?)`,
			username, passwordHash, now, now)
		return err
	}
	return nil
}

func (s *DB) UpdateAdminPassword(username, newPasswordHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	_, err := s.db.Exec(`UPDATE admin_account SET password_hash = ?, updated_at = ? WHERE username = ?`,
		newPasswordHash, now, username)
	return err
}

func (s *DB) RecordAuditLog(eventType, ip, userAgent, details string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`INSERT INTO audit_logs (event_type, ip_address, user_agent, details, created_at) VALUES (?, ?, ?, ?, ?)`,
		eventType, ip, userAgent, details, time.Now())
	return err
}

func (s *DB) GetAuditLogs(limit, offset int) ([]AuditLogRecord, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM audit_logs`).Scan(&total); err != nil {
		return nil, 0, err
	}

	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := s.db.Query(`SELECT id, event_type, ip_address, user_agent, details, created_at FROM audit_logs ORDER BY created_at DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []AuditLogRecord
	for rows.Next() {
		var log AuditLogRecord
		if err := rows.Scan(&log.ID, &log.EventType, &log.IPAddress, &log.UserAgent, &log.Details, &log.CreatedAt); err == nil {
			logs = append(logs, log)
		}
	}
	return logs, total, nil
}

func (s *DB) ClearAuditLogs() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM audit_logs`)
	return err
}

func (s *DB) IsIPBanned(ip string) (bool, *time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var bannedUntil sql.NullTime
	err := s.db.QueryRow(`SELECT banned_until FROM ip_bans WHERE ip = ?`, ip).Scan(&bannedUntil)
	if err == sql.ErrNoRows {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}

	if bannedUntil.Valid {
		if bannedUntil.Time.After(time.Now()) {
			return true, &bannedUntil.Time, nil
		}
		// Lazy cleanup: ban duration expired, auto-purge entry
		_, _ = s.db.Exec(`DELETE FROM ip_bans WHERE ip = ?`, ip)
	}
	return false, nil, nil
}

func (s *DB) RecordFailedLogin(ip string, maxAttempts int, banDuration time.Duration) (int, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	var attempts int
	var bannedUntil sql.NullTime
	var updatedAt time.Time

	err := s.db.QueryRow(`SELECT failed_attempts, banned_until, updated_at FROM ip_bans WHERE ip = ?`, ip).Scan(&attempts, &bannedUntil, &updatedAt)
	if err == sql.ErrNoRows || (err == nil && now.Sub(updatedAt) > banDuration && !bannedUntil.Valid) {
		// Fresh attempt or previous non-banned attempts aged out (> 15 mins)
		attempts = 1
		_, err = s.db.Exec(`INSERT OR REPLACE INTO ip_bans (ip, reason, failed_attempts, updated_at) VALUES (?, 'Failed login attempt', 1, ?)`, ip, now)
		return 1, false, err
	} else if err != nil {
		return 0, false, err
	}

	attempts++
	isBanned := false
	if attempts >= maxAttempts {
		isBanned = true
		banTime := now.Add(banDuration)
		_, err = s.db.Exec(`UPDATE ip_bans SET failed_attempts = ?, banned_until = ?, reason = 'Excessive failed login attempts', updated_at = ? WHERE ip = ?`,
			attempts, banTime, now, ip)
	} else {
		_, err = s.db.Exec(`UPDATE ip_bans SET failed_attempts = ?, updated_at = ? WHERE ip = ?`, attempts, now, ip)
	}

	return attempts, isBanned, err
}

func (s *DB) ClearFailedLogin(ip string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM ip_bans WHERE ip = ?`, ip)
	return err
}

func (s *DB) GetBannedIPs() ([]IPBanRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`SELECT ip, reason, banned_until, failed_attempts, updated_at FROM ip_bans ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []IPBanRecord
	for rows.Next() {
		var rec IPBanRecord
		var bannedUntil sql.NullTime
		if err := rows.Scan(&rec.IP, &rec.Reason, &bannedUntil, &rec.FailedAttempts, &rec.UpdatedAt); err == nil {
			if bannedUntil.Valid {
				rec.BannedUntil = &bannedUntil.Time
			}
			list = append(list, rec)
		}
	}
	return list, nil
}

func (s *DB) UnbanIP(ip string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM ip_bans WHERE ip = ?`, ip)
	return err
}


