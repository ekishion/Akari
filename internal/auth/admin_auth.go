package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
	"akari-bridge/internal/config"
	"akari-bridge/internal/storage"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrAccountLocked      = errors.New("account or IP is temporarily locked due to excessive failed attempts")
	ErrInvalidToken       = errors.New("invalid or expired admin token")
)

type AdminClaims struct {
	Username  string `json:"username"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

type AdminAuthService struct {
	cfg       *config.Config
	db        *storage.DB
	secretKey []byte
	mu        sync.RWMutex
}

func NewAdminAuthService(cfg *config.Config, db *storage.DB) *AdminAuthService {
	// Generate or retrieve persistent secret key for admin JWT signing
	secretKey := make([]byte, 32)
	if db != nil {
		if secretStr, err := db.GetSetting("admin_jwt_secret"); err == nil && secretStr != "" {
			if b, err := hex.DecodeString(secretStr); err == nil && len(b) == 32 {
				secretKey = b
			}
		}
	}
	// If still blank, generate new 256-bit random key and persist
	if secretKey[0] == 0 && secretKey[31] == 0 {
		_, _ = rand.Read(secretKey)
		if db != nil {
			_ = db.SaveSetting("admin_jwt_secret", hex.EncodeToString(secretKey))
		}
	}

	svc := &AdminAuthService{
		cfg:       cfg,
		db:        db,
		secretKey: secretKey,
	}

	// Ensure default admin account exists
	if db != nil {
		defaultPassHash, _ := svc.HashPassword("admin123")
		if err := db.EnsureDefaultAdmin("admin", defaultPassHash); err != nil {
			log.Printf("[AdminAuth] Warning: failed to ensure default admin: %v", err)
		}
	}

	return svc
}

func (s *AdminAuthService) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func (s *AdminAuthService) CheckPassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

func (s *AdminAuthService) Login(username, password, clientIP, userAgent string) (string, time.Time, error) {
	if s.db == nil {
		return "", time.Time{}, errors.New("database not available")
	}

	// 1. Check if IP is currently banned
	if banned, bannedUntil, err := s.db.IsIPBanned(clientIP); err == nil && banned {
		msg := "IP is temporarily locked"
		if bannedUntil != nil {
			msg = fmt.Sprintf("IP is locked until %s", bannedUntil.Format("15:04:05"))
		}
		_ = s.db.RecordAuditLog("login_blocked", clientIP, userAgent, "Blocked attempt from banned IP")
		return "", time.Time{}, fmt.Errorf("%s: %w", msg, ErrAccountLocked)
	}

	// 2. Fetch admin account
	acc, err := s.db.GetAdminAccount()
	if err != nil || acc == nil {
		_ = s.db.RecordAuditLog("login_failed", clientIP, userAgent, fmt.Sprintf("User '%s' not found", username))
		return "", time.Time{}, ErrInvalidCredentials
	}

	// 3. Verify username and password
	if !strings.EqualFold(acc.Username, username) || !s.CheckPassword(acc.PasswordHash, password) {
		attempts, isBanned, _ := s.db.RecordFailedLogin(clientIP, 5, 15*time.Minute)
		details := fmt.Sprintf("Failed password for user '%s' (Attempt %d/5)", username, attempts)
		if isBanned {
			details = fmt.Sprintf("IP banned for 15 mins after 5 failed attempts for user '%s'", username)
			_ = s.db.RecordAuditLog("ip_banned", clientIP, userAgent, details)
		} else {
			_ = s.db.RecordAuditLog("login_failed", clientIP, userAgent, details)
		}
		return "", time.Time{}, ErrInvalidCredentials
	}

	// 4. Reset failed attempts upon successful login
	_ = s.db.ClearFailedLogin(clientIP)

	// 5. Generate Admin JWT token (valid for 7 days)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	token, err := s.GenerateToken(acc.Username, expiresAt)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to generate token: %w", err)
	}

	_ = s.db.RecordAuditLog("login_success", clientIP, userAgent, fmt.Sprintf("Admin '%s' logged in successfully", username))
	return token, expiresAt, nil
}

func (s *AdminAuthService) ChangePassword(username, oldPassword, newPassword, clientIP, userAgent string) error {
	if s.db == nil {
		return errors.New("database not available")
	}

	if len(newPassword) < 6 {
		return errors.New("new password must be at least 6 characters")
	}

	acc, err := s.db.GetAdminAccount()
	if err != nil || acc == nil {
		return ErrInvalidCredentials
	}

	if !s.CheckPassword(acc.PasswordHash, oldPassword) {
		_ = s.db.RecordAuditLog("password_change_failed", clientIP, userAgent, "Incorrect old password")
		return errors.New("incorrect current password")
	}

	newHash, err := s.HashPassword(newPassword)
	if err != nil {
		return err
	}

	if err := s.db.UpdateAdminPassword(acc.Username, newHash); err != nil {
		return err
	}

	_ = s.db.RecordAuditLog("password_changed", clientIP, userAgent, fmt.Sprintf("Admin '%s' password updated", username))
	return nil
}

func (s *AdminAuthService) GenerateToken(username string, expiresAt time.Time) (string, error) {
	claims := AdminClaims{
		Username:  username,
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: expiresAt.Unix(),
	}

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	payloadB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)
	sig := s.signPayload(payloadB64)
	return payloadB64 + "." + sig, nil
}

func (s *AdminAuthService) ValidateToken(tokenString string) (*AdminClaims, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 2 {
		return nil, ErrInvalidToken
	}

	payloadB64, sig := parts[0], parts[1]
	expectedSig := s.signPayload(payloadB64)
	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return nil, ErrInvalidToken
	}

	claimsJSON, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return nil, ErrInvalidToken
	}

	var claims AdminClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	if claims.ExpiresAt < time.Now().Unix() {
		return nil, errors.New("token expired")
	}

	return &claims, nil
}

func (s *AdminAuthService) signPayload(payload string) string {
	mac := hmac.New(sha256.New, s.secretKey)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
