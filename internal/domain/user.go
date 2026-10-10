package domain

import "time"

// User represents an internal system or Emby user.
type User struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	PasswordHash   string     `json:"passwordHash,omitempty"`
	IsAdmin        bool       `json:"isAdmin"`
	IsDisabled     bool       `json:"isDisabled"`
	BgmAccessToken string     `json:"bgmAccessToken,omitempty"`
	BgmUserID      string     `json:"bgmUserId,omitempty"`
	LastLoginAt    *time.Time `json:"lastLoginAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
}

// PlaybackRecord tracks user playback state for an Emby item.
type PlaybackRecord struct {
	ItemId         string    `json:"itemId"`
	PositionTicks  int64     `json:"positionTicks"`
	TotalTicks     int64     `json:"totalTicks"`
	Played         bool      `json:"played"`
	PlayCount      int       `json:"playCount"`
	LastPlayedDate time.Time `json:"lastPlayedDate"`
}

// ActivityLog represents an administrative or system event.
type ActivityLog struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"` // e.g. "AUTH", "RULE", "CONFIG", "SYSTEM"
	Action    string    `json:"action"`
	Detail    string    `json:"detail"`
	IP        string    `json:"ip"`
	Timestamp time.Time `json:"timestamp"`
}

// IPBanRecord represents an IP ban state.
type IPBanRecord struct {
	IP             string     `json:"ip"`
	Reason         string     `json:"reason"`
	BannedUntil    *time.Time `json:"bannedUntil,omitempty"`
	FailedAttempts int        `json:"failedAttempts"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}
