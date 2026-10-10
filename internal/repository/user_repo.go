package repository

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"akari-bridge/internal/domain"
)

type UserRepository struct {
	conn *DBConn
}

func NewUserRepository(conn *DBConn) domain.IUserRepository {
	return &UserRepository{conn: conn}
}

func (r *UserRepository) CreateUser(u *domain.User, plainPassword string) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	var pwdHash string
	if plainPassword != "" {
		h := sha256.Sum256([]byte(plainPassword))
		pwdHash = hex.EncodeToString(h[:])
	} else if u.PasswordHash != "" {
		pwdHash = u.PasswordHash
	}

	isAdminInt := 0
	if u.IsAdmin {
		isAdminInt = 1
	}
	isDisabledInt := 0
	if u.IsDisabled {
		isDisabledInt = 1
	}

	bgmTokenEnc, _ := r.conn.Encrypt(u.BgmAccessToken)

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
	_, err := r.conn.db.Exec(query, u.ID, u.Name, pwdHash, isAdminInt, isDisabledInt, bgmTokenEnc, u.BgmUserID, u.LastLoginAt, u.CreatedAt)
	return err
}

func (r *UserRepository) GetUser(id string) (*domain.User, error) {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	if id == "00000000000000000000000000000001" || id == "" {
		id = "admin"
	}

	row := r.conn.db.QueryRow(`
		SELECT id, name, password_hash, is_admin, is_disabled, bgm_access_token, bgm_user_id, last_login_at, created_at
		FROM users WHERE id = ? OR name = ?
	`, id, id)

	var u domain.User
	var isAdminInt, isDisabledInt int
	var lastLoginAt sql.NullTime
	var bgmTokenEnc string

	err := row.Scan(&u.ID, &u.Name, &u.PasswordHash, &isAdminInt, &isDisabledInt, &bgmTokenEnc, &u.BgmUserID, &lastLoginAt, &u.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	u.IsAdmin = isAdminInt == 1
	u.IsDisabled = isDisabledInt == 1
	u.BgmAccessToken, _ = r.conn.Decrypt(bgmTokenEnc)
	if lastLoginAt.Valid {
		u.LastLoginAt = &lastLoginAt.Time
	}
	return &u, nil
}

func (r *UserRepository) GetUserByName(username string) (*domain.User, error) {
	return r.GetUser(username)
}

func (r *UserRepository) ListUsers() ([]domain.User, error) {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	rows, err := r.conn.db.Query(`
		SELECT id, name, password_hash, is_admin, is_disabled, bgm_access_token, bgm_user_id, last_login_at, created_at
		FROM users ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.User
	for rows.Next() {
		var u domain.User
		var isAdminInt, isDisabledInt int
		var lastLoginAt sql.NullTime
		var bgmTokenEnc string

		if err := rows.Scan(&u.ID, &u.Name, &u.PasswordHash, &isAdminInt, &isDisabledInt, &bgmTokenEnc, &u.BgmUserID, &lastLoginAt, &u.CreatedAt); err != nil {
			continue
		}
		u.IsAdmin = isAdminInt == 1
		u.IsDisabled = isDisabledInt == 1
		u.BgmAccessToken, _ = r.conn.Decrypt(bgmTokenEnc)
		if lastLoginAt.Valid {
			u.LastLoginAt = &lastLoginAt.Time
		}
		list = append(list, u)
	}
	return list, nil
}

func (r *UserRepository) UpdateUser(u *domain.User, newPassword string) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	var pwdHash string
	if newPassword != "" {
		h := sha256.Sum256([]byte(newPassword))
		pwdHash = hex.EncodeToString(h[:])
	} else {
		pwdHash = u.PasswordHash
	}

	isAdminInt := 0
	if u.IsAdmin {
		isAdminInt = 1
	}
	isDisabledInt := 0
	if u.IsDisabled {
		isDisabledInt = 1
	}

	bgmTokenEnc, _ := r.conn.Encrypt(u.BgmAccessToken)

	_, err := r.conn.db.Exec(`
		UPDATE users SET
			name = ?,
			password_hash = ?,
			is_admin = ?,
			is_disabled = ?,
			bgm_access_token = ?,
			bgm_user_id = ?,
			last_login_at = ?
		WHERE id = ?
	`, u.Name, pwdHash, isAdminInt, isDisabledInt, bgmTokenEnc, u.BgmUserID, u.LastLoginAt, u.ID)
	return err
}

func (r *UserRepository) DeleteUser(id string) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	_, err := r.conn.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}

func (r *UserRepository) UpdateUserLogin(id string, loginTime time.Time) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	_, err := r.conn.db.Exec(`UPDATE users SET last_login_at = ? WHERE id = ?`, loginTime, id)
	return err
}

func (r *UserRepository) SaveUserEasyPassword(id string, easyPin string) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	key := fmt.Sprintf("user_easy_pwd_%s", id)
	if easyPin == "" {
		_, err := r.conn.db.Exec(`DELETE FROM system_settings WHERE key = ?`, key)
		return err
	}
	h := sha256.Sum256([]byte(easyPin))
	hashHex := hex.EncodeToString(h[:])

	_, err := r.conn.db.Exec(`
		INSERT INTO system_settings (key, value, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, key, hashHex, time.Now())
	return err
}

func (r *UserRepository) VerifyUserEasyPassword(id string, easyPin string) bool {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	key := fmt.Sprintf("user_easy_pwd_%s", id)
	var hashHex string
	err := r.conn.db.QueryRow(`SELECT value FROM system_settings WHERE key = ?`, key).Scan(&hashHex)
	if err != nil || hashHex == "" {
		return false
	}
	h := sha256.Sum256([]byte(easyPin))
	return hex.EncodeToString(h[:]) == hashHex
}

func (r *UserRepository) SaveUserCustomToken(userId string, rawToken string) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	key := fmt.Sprintf("user_custom_token_%s", userId)
	if rawToken == "" {
		_, err := r.conn.db.Exec(`DELETE FROM system_settings WHERE key = ?`, key)
		return err
	}
	enc, err := r.conn.Encrypt(rawToken)
	if err != nil {
		return err
	}
	_, err = r.conn.db.Exec(`
		INSERT INTO system_settings (key, value, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, key, enc, time.Now())
	return err
}

func (r *UserRepository) GetUserCustomToken(userId string) (string, error) {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	key := fmt.Sprintf("user_custom_token_%s", userId)
	var enc string
	err := r.conn.db.QueryRow(`SELECT value FROM system_settings WHERE key = ?`, key).Scan(&enc)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return r.conn.Decrypt(enc)
}

func (r *UserRepository) GetUserByCustomToken(rawToken string) (*domain.User, error) {
	var targetUserId string
	func() {
		r.conn.mu.RLock()
		defer r.conn.mu.RUnlock()

		rows, err := r.conn.db.Query(`SELECT key, value FROM system_settings WHERE key LIKE 'user_custom_token_%'`)
		if err != nil {
			return
		}
		defer rows.Close()

		for rows.Next() {
			var key, enc string
			if err := rows.Scan(&key, &enc); err == nil {
				if token, err := r.conn.Decrypt(enc); err == nil && token == rawToken {
					targetUserId = strings.TrimPrefix(key, "user_custom_token_")
					break
				}
			}
		}
	}()

	if targetUserId != "" {
		return r.GetUser(targetUserId)
	}
	return nil, nil
}

func (r *UserRepository) CountUsers() (int, error) {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	var count int
	err := r.conn.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

func (r *UserRepository) IsIPBanned(ip string) (bool, *time.Time, error) {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	var bannedUntil sql.NullTime
	err := r.conn.db.QueryRow(`SELECT banned_until FROM ip_bans WHERE ip = ?`, ip).Scan(&bannedUntil)
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
		_, _ = r.conn.db.Exec(`DELETE FROM ip_bans WHERE ip = ?`, ip)
	}
	return false, nil, nil
}

func (r *UserRepository) RecordFailedLogin(ip string, maxAttempts int, banDuration time.Duration) (int, bool, error) {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	now := time.Now()
	var attempts int
	var bannedUntil sql.NullTime
	var updatedAt time.Time

	err := r.conn.db.QueryRow(`SELECT failed_attempts, banned_until, updated_at FROM ip_bans WHERE ip = ?`, ip).Scan(&attempts, &bannedUntil, &updatedAt)
	if err == sql.ErrNoRows || (err == nil && now.Sub(updatedAt) > banDuration && !bannedUntil.Valid) {
		attempts = 1
		_, err = r.conn.db.Exec(`INSERT OR REPLACE INTO ip_bans (ip, reason, failed_attempts, updated_at) VALUES (?, 'Failed login attempt', 1, ?)`, ip, now)
		return 1, false, err
	} else if err != nil {
		return 0, false, err
	}

	attempts++
	isBanned := false
	if attempts >= maxAttempts {
		isBanned = true
		banTime := now.Add(banDuration)
		_, err = r.conn.db.Exec(`UPDATE ip_bans SET failed_attempts = ?, banned_until = ?, reason = 'Excessive failed login attempts', updated_at = ? WHERE ip = ?`,
			attempts, banTime, now, ip)
	} else {
		_, err = r.conn.db.Exec(`UPDATE ip_bans SET failed_attempts = ?, updated_at = ? WHERE ip = ?`, attempts, now, ip)
	}

	return attempts, isBanned, err
}

func (r *UserRepository) ClearFailedLogin(ip string) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	_, err := r.conn.db.Exec(`DELETE FROM ip_bans WHERE ip = ?`, ip)
	return err
}

func (r *UserRepository) GetBannedIPs() ([]domain.IPBanRecord, error) {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	rows, err := r.conn.db.Query(`SELECT ip, reason, banned_until, failed_attempts, updated_at FROM ip_bans ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.IPBanRecord
	for rows.Next() {
		var rec domain.IPBanRecord
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

func (r *UserRepository) UnbanIP(ip string) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	_, err := r.conn.db.Exec(`DELETE FROM ip_bans WHERE ip = ?`, ip)
	return err
}

