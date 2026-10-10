package repository

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
	"akari-bridge/internal/config"
)

// DBConn wraps the SQLite database handle and master encryption key.
type DBConn struct {
	db     *sql.DB
	cfg    *config.Config
	encKey []byte
	mu     sync.RWMutex
}

// OpenDB opens or creates the SQLite database, performs schema migrations, and loads settings.
func OpenDB(cfg *config.Config) (*DBConn, error) {
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}

	dbPath := filepath.Join(cfg.DataDir, "bridge.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	conn := &DBConn{
		db:  db,
		cfg: cfg,
	}

	if err := conn.initSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to init db schema: %w", err)
	}

	// Load or generate AES encryption key (256-bit)
	encKey := make([]byte, 32)
	hasLoadedKey := false
	settingRepo := NewSettingRepository(conn)
	if secretHex, err := settingRepo.GetSetting("master_token_key"); err == nil && secretHex != "" {
		if b, err := hex.DecodeString(secretHex); err == nil && len(b) == 32 {
			encKey = b
			hasLoadedKey = true
		}
	}
	if !hasLoadedKey {
		_, _ = rand.Read(encKey)
		_ = settingRepo.SaveSetting("master_token_key", hex.EncodeToString(encKey))
	}
	conn.encKey = encKey

	// Load persisted system settings
	if bgmHost, err := settingRepo.GetSetting("bangumi_host"); err == nil && bgmHost != "" {
		cfg.BangumiHost = bgmHost
	}
	if danHost, err := settingRepo.GetSetting("dandan_host"); err == nil && danHost != "" {
		cfg.DanDanHost = danHost
	}
	if srvName, err := settingRepo.GetSetting("server_name"); err == nil && srvName != "" {
		cfg.ServerName = srvName
	}

	log.Printf("[DB] SQLite database initialized at %s", dbPath)
	return conn, nil
}

// Close closes the underlying database connection.
func (c *DBConn) Close() error {
	return c.db.Close()
}

// DB returns the underlying sql.DB handle.
func (c *DBConn) DB() *sql.DB {
	return c.db
}

// Config returns the active application configuration.
func (c *DBConn) Config() *config.Config {
	return c.cfg
}

// Encrypt encrypts a string using AES-GCM.
func (c *DBConn) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	block, err := aes.NewCipher(c.encKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a hex-encoded AES-GCM ciphertext string.
func (c *DBConn) Decrypt(cipherHex string) (string, error) {
	if cipherHex == "" {
		return "", nil
	}
	data, err := hex.DecodeString(cipherHex)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(c.encKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func (c *DBConn) initSchema() error {
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

	if _, err := c.db.Exec(schema); err != nil {
		return err
	}

	// Upgrade legacy schema
	_, _ = c.db.Exec(`ALTER TABLE playback_histories ADD COLUMN is_favorite INTEGER DEFAULT 0`)
	return nil
}
