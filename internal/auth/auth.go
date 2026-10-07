package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"akari-bridge/internal/config"
	"akari-bridge/internal/model"
	"akari-bridge/internal/storage"
)

const (
	DefaultAdminID = "admin"
	LegacyAdminID  = "00000000000000000000000000000001"
)

type AuthService struct {
	cfg      *config.Config
	db       *storage.DB
	mu       sync.RWMutex
	tokens   map[string]string // in-memory cache: token -> userId
	users    map[string]*model.UserDto
	userPwds map[string]string // userId -> password
}

func NewAuthService(cfg *config.Config, db *storage.DB) *AuthService {
	svc := &AuthService{
		cfg:      cfg,
		db:       db,
		tokens:   make(map[string]string),
		users:    make(map[string]*model.UserDto),
		userPwds: make(map[string]string),
	}

	adminUsername := cfg.AdminUsername
	if adminUsername == "" {
		adminUsername = "Admin"
	}
	adminId := strings.ToLower(adminUsername)
	if adminId == "" {
		adminId = DefaultAdminID
	}

	hasPwd := cfg.AdminPassword != ""
	now := time.Now()

	adminUser := &model.UserDto{
		Name:                  adminUsername,
		ServerId:              cfg.ServerId,
		Id:                    adminId,
		HasPassword:           hasPwd,
		HasConfiguredPassword: hasPwd,
		EnableAutoLogin:       !hasPwd,
		LastLoginDate:         &now,
		Configuration: model.UserConfiguration{
			PlayDefaultAudioTrack:      true,
			SubtitleLanguagePreference: "chi",
			DisplayMissingEpisodes:     false,
			EnableNextEpisodeAutoPlay:  true,
			GroupedFolders:             []string{},
			MyMediaExcludes:            []string{},
			OrderedViews:               []string{},
			LatestItemsExcludes:        []string{},
			HidePlayedInLatest:         false,
			RememberAudioSelections:    true,
			RememberSubtitleSelections: true,
		},
		Policy: model.UserPolicy{
			IsAdministrator:     true,
			IsDisabled:          false,
			IsHidden:            false,
			EnableMediaPlayback: true,
			EnableAllDevices:    true,
			EnableAllChannels:   true,
			EnableAllFolders:    true,
			EnableAllLibraries:  true,
			EnabledFolders:      []string{},
			EnabledLibraries:    []string{},
			BlockedMediaFolders: []string{},
			BlockedChannels:     []string{},
		},
	}
	svc.users[adminId] = adminUser
	svc.userPwds[adminId] = cfg.AdminPassword

	// Load existing users from SQLite if DB is present
	if db != nil {
		if dbUsers, err := db.ListUsers(); err == nil && len(dbUsers) > 0 {
			for _, du := range dbUsers {
				hasUserPwd := du.PasswordHash != ""
				dto := &model.UserDto{
					Name:                  du.Name,
					ServerId:              cfg.ServerId,
					Id:                    du.ID,
					HasPassword:           hasUserPwd,
					HasConfiguredPassword: hasUserPwd,
					EnableAutoLogin:       !hasUserPwd,
					LastLoginDate:         du.LastLoginAt,
					Configuration: model.UserConfiguration{
						PlayDefaultAudioTrack:      true,
						SubtitleLanguagePreference: "chi",
						DisplayMissingEpisodes:     false,
						EnableNextEpisodeAutoPlay:  true,
						GroupedFolders:             []string{},
						MyMediaExcludes:            []string{},
						OrderedViews:               []string{},
						LatestItemsExcludes:        []string{},
						HidePlayedInLatest:         false,
						RememberAudioSelections:    true,
						RememberSubtitleSelections: true,
					},
					Policy: model.UserPolicy{
						IsAdministrator:     du.IsAdmin,
						IsDisabled:          du.IsDisabled,
						IsHidden:            false,
						EnableMediaPlayback: true,
						EnableAllDevices:    true,
						EnableAllChannels:   true,
						EnableAllFolders:    true,
						EnableAllLibraries:  true,
						EnabledFolders:      []string{},
						EnabledLibraries:    []string{},
						BlockedMediaFolders: []string{},
						BlockedChannels:     []string{},
					},
				}
				svc.users[du.ID] = dto
				svc.userPwds[du.ID] = du.PasswordHash
			}
			log.Printf("[AuthService] Loaded %d user(s) from SQLite", len(dbUsers))
		}

		// Ensure admin user is persisted in DB
		_ = db.UpsertUser(&storage.UserRecord{
			ID:           adminId,
			Name:         adminUsername,
			PasswordHash: cfg.AdminPassword,
			IsAdmin:      true,
			LastLoginAt:  &now,
			CreatedAt:    now,
		})
	}

	return svc
}

func (s *AuthService) Authenticate(username, password string) (*model.AuthenticationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var matchedUser *model.UserDto
	for _, u := range s.users {
		if strings.EqualFold(u.Name, username) || strings.EqualFold(u.Id, username) {
			matchedUser = u
			break
		}
	}

	// If no username provided, default to admin if single user or matching admin
	if matchedUser == nil && (username == "" || strings.EqualFold(username, s.cfg.AdminUsername)) {
		for _, u := range s.users {
			if u.Policy.IsAdministrator {
				matchedUser = u
				break
			}
		}
	}

	if matchedUser == nil {
		return nil, gin.Error{Err: http.ErrAbortHandler, Type: gin.ErrorTypePublic}
	}

	expectedPwd := s.userPwds[matchedUser.Id]
	if expectedPwd != "" && password != expectedPwd {
		return nil, gin.Error{Err: http.ErrAbortHandler, Type: gin.ErrorTypePublic}
	}

	// Generate a 32-char hex token
	bytes := make([]byte, 16)
	_, _ = rand.Read(bytes)
	token := hex.EncodeToString(bytes)

	s.tokens[token] = matchedUser.Id

	now := time.Now()
	matchedUser.LastLoginDate = &now

	if s.db != nil {
		_ = s.db.SaveToken(token, matchedUser.Id, "EmbyClient", "", nil)
		_ = s.db.UpsertUser(&storage.UserRecord{
			ID:           matchedUser.Id,
			Name:         matchedUser.Name,
			PasswordHash: expectedPwd,
			IsAdmin:      matchedUser.Policy.IsAdministrator,
			LastLoginAt:  &now,
		})
	}

	return &model.AuthenticationResult{
		User:        *matchedUser,
		AccessToken: token,
		ServerId:    s.cfg.ServerId,
	}, nil
}

func (s *AuthService) GetUser(userId string) (*model.UserDto, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if userId == LegacyAdminID || userId == "" {
		userId = DefaultAdminID
	}

	user, ok := s.users[userId]
	if !ok {
		// Case-insensitive lookup
		for _, u := range s.users {
			if strings.EqualFold(u.Id, userId) || strings.EqualFold(u.Name, userId) {
				return u, true
			}
		}
	}
	return user, ok
}

func (s *AuthService) GetPublicUsers() []model.UserDto {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]model.UserDto, 0, len(s.users))
	for _, u := range s.users {
		if !u.Policy.IsDisabled {
			res = append(res, *u)
		}
	}
	return res
}

func (s *AuthService) ValidateToken(token string) (*model.UserDto, bool) {
	s.mu.RLock()
	userId, ok := s.tokens[token]
	s.mu.RUnlock()

	if !ok && s.db != nil && token != "" {
		if dbUserId, err := s.db.GetUserIDByToken(token); err == nil && dbUserId != "" {
			userId = dbUserId
			ok = true
			s.mu.Lock()
			s.tokens[token] = dbUserId
			s.mu.Unlock()
		}
	}

	if !ok {
		// Single user open mode if admin has no password
		if s.cfg.AdminPassword == "" {
			for _, u := range s.users {
				if u.Policy.IsAdministrator {
					return u, true
				}
			}
		}
		return nil, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.users[userId]
	return user, ok
}

func (s *AuthService) CreateUser(username, password string, isAdmin bool) (*model.UserDto, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	username = strings.TrimSpace(username)
	if username == "" {
		return nil, fmt.Errorf("username cannot be empty")
	}

	id := strings.ToLower(username)
	if _, exists := s.users[id]; exists {
		return nil, fmt.Errorf("user '%s' already exists", username)
	}

	hasPwd := password != ""
	now := time.Now()
	user := &model.UserDto{
		Name:                  username,
		ServerId:              s.cfg.ServerId,
		Id:                    id,
		HasPassword:           hasPwd,
		HasConfiguredPassword: hasPwd,
		EnableAutoLogin:       !hasPwd,
		LastLoginDate:         &now,
		Configuration: model.UserConfiguration{
			PlayDefaultAudioTrack:      true,
			SubtitleLanguagePreference: "chi",
			DisplayMissingEpisodes:     false,
			EnableNextEpisodeAutoPlay:  true,
			GroupedFolders:             []string{},
			MyMediaExcludes:            []string{},
			OrderedViews:               []string{},
			LatestItemsExcludes:        []string{},
			HidePlayedInLatest:         false,
			RememberAudioSelections:    true,
			RememberSubtitleSelections: true,
		},
		Policy: model.UserPolicy{
			IsAdministrator:     isAdmin,
			IsDisabled:          false,
			IsHidden:            false,
			EnableMediaPlayback: true,
			EnableAllDevices:    true,
			EnableAllChannels:   true,
			EnableAllFolders:    true,
			EnableAllLibraries:  true,
			EnabledFolders:      []string{},
			EnabledLibraries:    []string{},
			BlockedMediaFolders: []string{},
			BlockedChannels:     []string{},
		},
	}

	s.users[id] = user
	s.userPwds[id] = password

	if s.db != nil {
		_ = s.db.UpsertUser(&storage.UserRecord{
			ID:           id,
			Name:         username,
			PasswordHash: password,
			IsAdmin:      isAdmin,
			CreatedAt:    now,
		})
	}

	return user, nil
}

func (s *AuthService) UpdatePassword(userId, newPassword string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userId == LegacyAdminID {
		userId = DefaultAdminID
	}

	user, ok := s.users[userId]
	if !ok {
		return fmt.Errorf("user not found")
	}

	hasPwd := newPassword != ""
	user.HasPassword = hasPwd
	user.HasConfiguredPassword = hasPwd
	user.EnableAutoLogin = !hasPwd
	s.userPwds[userId] = newPassword

	if s.db != nil {
		_ = s.db.UpsertUser(&storage.UserRecord{
			ID:           userId,
			Name:         user.Name,
			PasswordHash: newPassword,
			IsAdmin:      user.Policy.IsAdministrator,
		})
	}

	return nil
}

func (s *AuthService) DeleteUser(userId string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userId == LegacyAdminID || userId == DefaultAdminID {
		return fmt.Errorf("cannot delete default administrator")
	}

	delete(s.users, userId)
	delete(s.userPwds, userId)

	// Clean up tokens
	for token, uid := range s.tokens {
		if uid == userId {
			delete(s.tokens, token)
		}
	}

	if s.db != nil {
		_ = s.db.DeleteUser(userId)
		_ = s.db.RevokeAllUserTokens(userId)
	}

	return nil
}

func (s *AuthService) ListUsers() []*model.UserDto {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*model.UserDto, 0, len(s.users))
	for _, u := range s.users {
		list = append(list, u)
	}
	return list
}

func (s *AuthService) GetDB() *storage.DB {
	return s.db
}

func (s *AuthService) CreateTokenForUser(userId, clientName string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userId == LegacyAdminID || userId == "" {
		userId = DefaultAdminID
	}

	if _, ok := s.users[userId]; !ok {
		return "", fmt.Errorf("user '%s' not found", userId)
	}

	bytes := make([]byte, 16)
	_, _ = rand.Read(bytes)
	token := hex.EncodeToString(bytes)

	s.tokens[token] = userId

	if s.db != nil {
		if err := s.db.SaveToken(token, userId, clientName, "", nil); err != nil {
			return "", err
		}
	}

	return token, nil
}

func (s *AuthService) RevokeToken(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.tokens, token)
	if s.db != nil {
		return s.db.RevokeToken(token)
	}
	return nil
}

func (s *AuthService) GetUserTokens(userId string) ([]*storage.TokenRecord, error) {
	if s.db == nil {
		return nil, nil
	}
	return s.db.GetTokensByUserID(userId)
}

func (s *AuthService) UpdateUserBangumi(userId, bgmAccessToken, bgmUserId string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userId == LegacyAdminID || userId == "" {
		userId = DefaultAdminID
	}

	user, ok := s.users[userId]
	if !ok {
		return fmt.Errorf("user not found")
	}

	if s.db != nil {
		existing, _ := s.db.GetUserByID(userId)
		pwd := s.userPwds[userId]
		isAdmin := user.Policy.IsAdministrator
		isDisabled := user.Policy.IsDisabled
		if existing != nil {
			pwd = existing.PasswordHash
			isAdmin = existing.IsAdmin
			isDisabled = existing.IsDisabled
		}
		return s.db.UpsertUser(&storage.UserRecord{
			ID:             userId,
			Name:           user.Name,
			PasswordHash:   pwd,
			IsAdmin:        isAdmin,
			IsDisabled:     isDisabled,
			BgmAccessToken: bgmAccessToken,
			BgmUserID:      bgmUserId,
		})
	}

	return nil
}

func (s *AuthService) GetUserBangumi(userId string) (string, string) {
	if userId == LegacyAdminID || userId == "" {
		userId = DefaultAdminID
	}
	if s.db != nil {
		if rec, err := s.db.GetUserByID(userId); err == nil && rec != nil {
			return rec.BgmAccessToken, rec.BgmUserID
		}
	}
	return "", ""
}


// Middleware creates a Gin middleware that extracts and validates Emby tokens
func (s *AuthService) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := ExtractToken(c)

		// Public endpoints don't require token
		path := c.Request.URL.Path
		if isPublicPath(path) {
			c.Next()
			return
		}

		user, valid := s.ValidateToken(token)
		if !valid && s.cfg.AdminPassword != "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		if user != nil {
			c.Set("user", user)
			c.Set("userId", user.Id)
		}
		c.Next()
	}
}

func ExtractToken(c *gin.Context) string {
	if key := c.Query("api_key"); key != "" {
		return key
	}
	if token := c.GetHeader("X-Emby-Token"); token != "" {
		return token
	}
	if token := c.GetHeader("X-MediaBrowser-Token"); token != "" {
		return token
	}
	if auth := c.GetHeader("X-Emby-Authorization"); auth != "" {
		parts := strings.Split(auth, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(strings.ToLower(part), "token=") {
				tokenVal := strings.TrimPrefix(part, "token=")
				tokenVal = strings.TrimPrefix(part, "Token=")
				return strings.Trim(tokenVal, "\"")
			}
		}
	}
	return ""
}

func isPublicPath(path string) bool {
	publicPrefixes := []string{
		"/emby/System/Info/Public",
		"/emby/System/Ping",
		"/emby/Users/AuthenticateByName",
		"/emby/Users/Public",
		"/emby/items/images",
	}
	for _, p := range publicPrefixes {
		if strings.HasPrefix(strings.ToLower(path), strings.ToLower(p)) {
			return true
		}
	}
	return false
}
