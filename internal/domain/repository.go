package domain

import (
	"time"

	"akari-bridge/internal/engine"
)

// IUserRepository manages user lifecycle, credentials, and tokens.
type IUserRepository interface {
	CreateUser(user *User, plainPassword string) error
	GetUser(id string) (*User, error)
	GetUserByName(username string) (*User, error)
	ListUsers() ([]User, error)
	UpdateUser(user *User, newPassword string) error
	DeleteUser(id string) error
	UpdateUserLogin(id string, loginTime time.Time) error
	SaveUserEasyPassword(id string, easyPin string) error
	VerifyUserEasyPassword(id string, easyPin string) bool
	SaveUserCustomToken(userId string, rawToken string) error
	GetUserCustomToken(userId string) (string, error)
	GetUserByCustomToken(rawToken string) (*User, error)
	CountUsers() (int, error)
	IsIPBanned(ip string) (bool, *time.Time, error)
	RecordFailedLogin(ip string, maxAttempts int, banDuration time.Duration) (int, bool, error)
	ClearFailedLogin(ip string) error
	GetBannedIPs() ([]IPBanRecord, error)
	UnbanIP(ip string) error
}

// IPlaybackRepository manages playback progress, durations, and resume markers.
type IPlaybackRepository interface {
	SaveItemDuration(itemId string, totalTicks int64) error
	GetItemTotalTicks(itemId string) int64
	UpdatePlaybackProgress(userId, itemId string, posTicks, totalTicks int64) error
	GetPlaybackProgress(userId, itemId string) (int64, int64, bool)
	ListUserPlaybackRecords(userId string) ([]PlaybackRecord, error)
	GetProgress(userId, itemId string) (*PlaybackRecord, error)
	MarkPlayed(userId, itemId string, played bool) error
	GetUserWatchingSubjectIDs(userId string, limit int) []int
	GetUserResumeItems(userId string, limit int) []string
	ListRecentHistories(limit int) ([]PlaybackRecord, error)
}

// IFavoriteRepository manages user favorited items.
type IFavoriteRepository interface {
	ToggleFavoriteItem(userId, itemId string, isFav bool) error
	IsFavoriteItem(userId, itemId string) bool
	ListFavoriteItems(userId string) ([]string, error)
	MarkFavorite(userId, itemId string, isFav bool) error
	IsFavorite(userId, itemId string) (bool, error)
	UnmarkAllSubjectFavorites(userId string, subjectId int) error
}

// ISettingRepository manages encrypted/plain configuration key-values.
type ISettingRepository interface {
	GetSetting(key string) (string, error)
	SaveSetting(key, val string) error
}

// IRuleRepository manages Kazumi rule plugins persistence.
type IRuleRepository interface {
	GetAllPlugins() ([]engine.Plugin, error)
	GetPluginByName(name string) (*engine.Plugin, error)
	SavePlugin(p *engine.Plugin) error
	DeletePlugin(name string) error
	TogglePlugin(name string, enabled bool) error
	ImportPluginsJSON(data []byte) (int, error)
	ExportPluginsJSON() ([]byte, error)
}

// ISynonymRepository manages search title synonyms and Bangumi subject aliases.
type ISynonymRepository interface {
	GetGlobalSynonyms() (map[string]string, error)
	SaveGlobalSynonyms(synonyms map[string]string) error
	GetSubjectAliases(subId int) ([]string, error)
	SaveSubjectAliases(subId int, aliases []string) error
	ListGlobalSynonyms() ([]map[string]any, error)
	UpsertGlobalSynonym(pattern, replacement string, enabled bool) error
	DeleteGlobalSynonymByPattern(pattern string) error
	ResetDefaultGlobalSynonyms() error
	ListSubjectAliases() ([]map[string]any, error)
	UpsertSubjectAliases(subjectId int, title string, aliases []string) error
	DeleteSubjectAliases(subjectId int) error
}

// IActivityRepository manages system and administrative activity audit logs.
type IActivityRepository interface {
	AddActivityLog(logType, action, detail, ip string) error
	GetRecentActivityLogs(limit int) ([]ActivityLog, error)
	RecordAuditLog(action, ip, userAgent, details string) error
	GetAuditLogs(limit, offset int) ([]ActivityLog, int, error)
	ClearAuditLogs() error
}
