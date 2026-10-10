package bilibili

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type qrSession struct {
	QRCodeKey string
	URL       string
	CreatedAt time.Time
}

type AuthManager struct {
	client         *Client
	mu             sync.RWMutex
	creds          *BilibiliCredentials
	settings       *BilibiliSettings
	qrSessions     map[string]qrSession
	cachedStatus   *BilibiliStatus
	cachedStatusAt time.Time
	saveCredsFunc  func(*BilibiliCredentials) error
	saveConfigFunc func(*BilibiliSettings) error
}

func NewAuthManager(
	client *Client,
	initialCreds *BilibiliCredentials,
	initialSettings *BilibiliSettings,
	saveCredsFunc func(*BilibiliCredentials) error,
	saveConfigFunc func(*BilibiliSettings) error,
) *AuthManager {
	if client == nil {
		client = NewClient(nil)
	}
	if initialSettings == nil {
		initialSettings = &BilibiliSettings{
			Enabled:        true,
			PreferBilibili: true,
			MaxQuality:     120,
		}
	}
	return &AuthManager{
		client:         client,
		creds:          initialCreds,
		settings:       initialSettings,
		qrSessions:     make(map[string]qrSession),
		saveCredsFunc:  saveCredsFunc,
		saveConfigFunc: saveConfigFunc,
	}
}

// GetCredentials returns current Bilibili credentials (thread-safe)
func (a *AuthManager) GetCredentials() *BilibiliCredentials {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.creds == nil {
		return nil
	}
	copy := *a.creds
	return &copy
}

// SetCredentials updates credentials and persists them
func (a *AuthManager) SetCredentials(creds *BilibiliCredentials) error {
	a.mu.Lock()
	a.creds = creds
	a.cachedStatus = nil // invalidate cache
	a.mu.Unlock()

	if a.saveCredsFunc != nil && creds != nil {
		return a.saveCredsFunc(creds)
	}
	return nil
}

// ClearCredentials logs out the user
func (a *AuthManager) ClearCredentials() error {
	a.mu.Lock()
	a.creds = nil
	a.cachedStatus = nil // invalidate cache
	a.mu.Unlock()

	if a.saveCredsFunc != nil {
		return a.saveCredsFunc(&BilibiliCredentials{})
	}
	return nil
}

// GetSettings returns current settings (thread-safe)
func (a *AuthManager) GetSettings() *BilibiliSettings {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.settings == nil {
		return &BilibiliSettings{Enabled: true, PreferBilibili: true, MaxQuality: 120}
	}
	copy := *a.settings
	return &copy
}

// SetSettings updates settings and persists them
func (a *AuthManager) SetSettings(settings *BilibiliSettings) error {
	a.mu.Lock()
	a.settings = settings
	a.cachedStatus = nil // invalidate cache
	a.mu.Unlock()

	if a.saveConfigFunc != nil && settings != nil {
		return a.saveConfigFunc(settings)
	}
	return nil
}

// GenerateQR creates a new QR session
func (a *AuthManager) GenerateQR(ctx context.Context) (*QRGenerateResponse, error) {
	resp, err := a.client.GenerateQRCode(ctx)
	if err != nil {
		return nil, err
	}
	if resp.Code != 0 || resp.Data.QRCodeKey == "" {
		return nil, fmt.Errorf("bilibili QR generate failed: %s (code %d)", resp.Message, resp.Code)
	}

	a.mu.Lock()
	a.qrSessions[resp.Data.QRCodeKey] = qrSession{
		QRCodeKey: resp.Data.QRCodeKey,
		URL:       resp.Data.URL,
		CreatedAt: time.Now(),
	}
	// Clean up old sessions (>10 mins)
	for k, v := range a.qrSessions {
		if time.Since(v.CreatedAt) > 10*time.Minute {
			delete(a.qrSessions, k)
		}
	}
	a.mu.Unlock()

	return resp, nil
}

// PollQR checks the status of a QR login session and saves credentials on success
func (a *AuthManager) PollQR(ctx context.Context, qrcodeKey string) (*QRPollResponse, bool, error) {
	pollResp, creds, err := a.client.PollQRCode(ctx, qrcodeKey)
	if err != nil {
		return nil, false, err
	}

	if pollResp.Data.Code == 0 && creds != nil && creds.SessData != "" {
		// Login Success!
		if err := a.SetCredentials(creds); err != nil {
			return pollResp, true, fmt.Errorf("failed to save credentials: %w", err)
		}

		a.mu.Lock()
		delete(a.qrSessions, qrcodeKey)
		a.cachedStatus = nil
		a.mu.Unlock()

		return pollResp, true, nil
	}

	return pollResp, false, nil
}

// GetStatus returns user status (with in-memory caching to avoid frequent roundtrips)
func (a *AuthManager) GetStatus(ctx context.Context) *BilibiliStatus {
	a.mu.RLock()
	if a.cachedStatus != nil && time.Since(a.cachedStatusAt) < 5*time.Minute {
		cached := *a.cachedStatus
		a.mu.RUnlock()
		return &cached
	}
	a.mu.RUnlock()

	creds := a.GetCredentials()
	settings := a.GetSettings()
	st := a.client.GetStatus(ctx, creds, settings)

	if st != nil {
		a.mu.Lock()
		a.cachedStatus = st
		a.cachedStatusAt = time.Now()
		a.mu.Unlock()
	}

	return st
}
