package embyapi

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"akari-bridge/internal/auth"
	"akari-bridge/internal/mapper"
	"akari-bridge/internal/model"
	"akari-bridge/internal/storage"
)

type UsersHandler struct {
	authService *auth.AuthService
	db          *storage.DB
}

func NewUsersHandler(authService *auth.AuthService, db *storage.DB) *UsersHandler {
	return &UsersHandler{
		authService: authService,
		db:          db,
	}
}

// AuthenticateByName handles POST /emby/Users/AuthenticateByName
func (h *UsersHandler) AuthenticateByName(c *gin.Context) {
	clientIP := c.ClientIP()
	userAgent := c.Request.UserAgent()

	// 1. IP Ban check
	if h.db != nil {
		if banned, bannedUntil, err := h.db.IsIPBanned(clientIP); err == nil && banned {
			msg := "IP is temporarily locked due to excessive failed attempts"
			if bannedUntil != nil {
				msg = fmt.Sprintf("IP is locked until %s", bannedUntil.Format("15:04:05"))
			}
			_ = h.db.RecordAuditLog("login_blocked", clientIP, userAgent, "Blocked attempt from banned IP")
			c.JSON(http.StatusTooManyRequests, gin.H{"error": msg})
			return
		}
	}

	var req model.AuthenticateUserByNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	pwd := req.Pw
	if pwd == "" {
		pwd = req.Password
	}

	result, err := h.authService.Authenticate(req.Username, pwd)
	if err != nil {
		if h.db != nil {
			attempts, isBanned, _ := h.db.RecordFailedLogin(clientIP, 5, 15*time.Minute)
			details := fmt.Sprintf("Failed password for user '%s' (Attempt %d/5)", req.Username, attempts)
			if isBanned {
				details = fmt.Sprintf("IP banned for 15 mins after 5 failed attempts for user '%s'", req.Username)
				_ = h.db.RecordAuditLog("ip_banned", clientIP, userAgent, details)
			} else {
				_ = h.db.RecordAuditLog("login_failed", clientIP, userAgent, details)
			}
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	if h.db != nil {
		_ = h.db.ClearFailedLogin(clientIP)
		_ = h.db.RecordAuditLog("login_success", clientIP, userAgent, fmt.Sprintf("User '%s' authenticated via Emby API", req.Username))
	}

	c.JSON(http.StatusOK, result)
}

// GetUser handles GET /emby/Users/:id
func (h *UsersHandler) GetUser(c *gin.Context) {
	userId := c.Param("id")
	user, ok := h.authService.GetUser(userId)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}

// GetPublicUsers handles GET /emby/Users/Public
func (h *UsersHandler) GetPublicUsers(c *gin.Context) {
	users := h.authService.GetPublicUsers()
	c.JSON(http.StatusOK, users)
}

// GetCurrentItemUserData handles GET /emby/Users/:id/Items/:itemId/UserData
func (h *UsersHandler) GetCurrentItemUserData(c *gin.Context) {
	userId := c.Param("id")
	itemId := c.Param("itemId")

	if h.db != nil {
		c.JSON(http.StatusOK, h.db.GetUserItemData(userId, itemId))
		return
	}
	c.JSON(http.StatusOK, &model.UserItemDataDto{Key: itemId})
}

// MarkPlayedItem handles POST /emby/Users/:id/PlayedItems/:itemId
func (h *UsersHandler) MarkPlayedItem(c *gin.Context) {
	userId := c.Param("id")
	if userId == "" {
		userId = c.Param("userId")
	}
	itemId := c.Param("itemId")

	if h.db != nil {
		_ = h.db.MarkPlayed(userId, itemId, true)
		c.JSON(http.StatusOK, h.db.GetUserItemData(userId, itemId))
		return
	}
	c.JSON(http.StatusOK, &model.UserItemDataDto{Key: itemId, Played: true})
}

// UnmarkPlayedItem handles DELETE /emby/Users/:id/PlayedItems/:itemId
func (h *UsersHandler) UnmarkPlayedItem(c *gin.Context) {
	userId := c.Param("id")
	if userId == "" {
		userId = c.Param("userId")
	}
	itemId := c.Param("itemId")

	if h.db != nil {
		_ = h.db.MarkPlayed(userId, itemId, false)
		c.JSON(http.StatusOK, h.db.GetUserItemData(userId, itemId))
		return
	}
	c.JSON(http.StatusOK, &model.UserItemDataDto{Key: itemId, Played: false})
}

// MarkFavoriteItem handles POST /emby/Users/:id/FavoriteItems/:itemId
func (h *UsersHandler) MarkFavoriteItem(c *gin.Context) {
	userId := c.Param("id")
	if userId == "" {
		userId = c.GetString("userId")
	}
	if userId == "" {
		userId = "admin"
	}
	itemId := c.Param("itemId")

	if h.db != nil {
		_ = h.db.MarkFavorite(userId, itemId, true)
		// If favoriting an episode or season, also cascade favorite to the parent series!
		if strings.HasPrefix(itemId, "bgm_ep_") || strings.HasPrefix(itemId, "bgm_season_") {
			subId := mapper.ExtractSubjectId(itemId)
			if subId > 0 {
				_ = h.db.MarkFavorite(userId, fmt.Sprintf("bgm_sub_%d", subId), true)
			}
		}
		c.JSON(http.StatusOK, h.db.GetUserItemData(userId, itemId))
		return
	}
	c.JSON(http.StatusOK, &model.UserItemDataDto{Key: itemId, IsFavorite: true})
}

// UnmarkFavoriteItem handles DELETE /emby/Users/:id/FavoriteItems/:itemId
func (h *UsersHandler) UnmarkFavoriteItem(c *gin.Context) {
	userId := c.Param("id")
	if userId == "" {
		userId = c.GetString("userId")
	}
	if userId == "" {
		userId = "admin"
	}
	itemId := c.Param("itemId")

	if h.db != nil {
		_ = h.db.MarkFavorite(userId, itemId, false)
		if strings.HasPrefix(itemId, "bgm_sub_") {
			subId := mapper.ExtractSubjectId(itemId)
			if subId > 0 {
				_ = h.db.UnmarkAllSubjectFavorites(userId, subId)
			}
		}
		c.JSON(http.StatusOK, h.db.GetUserItemData(userId, itemId))
		return
	}
	c.JSON(http.StatusOK, &model.UserItemDataDto{Key: itemId, IsFavorite: false})
}

