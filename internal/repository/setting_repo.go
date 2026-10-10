package repository

import (
	"time"

	"akari-bridge/internal/domain"
)

type SettingRepository struct {
	conn *DBConn
}

func NewSettingRepository(conn *DBConn) domain.ISettingRepository {
	return &SettingRepository{conn: conn}
}

func (r *SettingRepository) GetSetting(key string) (string, error) {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	var val string
	err := r.conn.db.QueryRow(`SELECT value FROM system_settings WHERE key = ?`, key).Scan(&val)
	if err != nil {
		return "", err
	}
	return val, nil
}

func (r *SettingRepository) SaveSetting(key, val string) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	_, err := r.conn.db.Exec(`
		INSERT INTO system_settings (key, value, updated_at) 
		VALUES (?, ?, ?) 
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, val, time.Now())
	return err
}
