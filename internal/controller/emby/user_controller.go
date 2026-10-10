package emby

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"akari-bridge/internal/auth"
	"akari-bridge/internal/domain"
	"akari-bridge/internal/mapper"
	"akari-bridge/internal/model"
)

type UserController struct {
	authService  *auth.AuthService
	playbackRepo domain.IPlaybackRepository
	favRepo      domain.IFavoriteRepository
	userRepo     domain.IUserRepository
	activityRepo domain.IActivityRepository
}

func NewUserController(
	authService *auth.AuthService,
	playbackRepo domain.IPlaybackRepository,
	favRepo domain.IFavoriteRepository,
	userRepo domain.IUserRepository,
	activityRepo domain.IActivityRepository,
) *UserController {
	return &UserController{
		authService:  authService,
		playbackRepo: playbackRepo,
		favRepo:      favRepo,
		userRepo:     userRepo,
		activityRepo: activityRepo,
	}
}

// AuthenticateByName handles POST /emby/Users/AuthenticateByName
func (ctrl *UserController) AuthenticateByName(c *gin.Context) {
	clientIP := c.ClientIP()
	userAgent := c.Request.UserAgent()

	// 1. IP Ban check
	if ctrl.userRepo != nil {
		if banned, bannedUntil, err := ctrl.userRepo.IsIPBanned(clientIP); err == nil && banned {
			msg := "IP is temporarily locked due to excessive failed attempts"
			if bannedUntil != nil {
				msg = fmt.Sprintf("IP is locked until %s", bannedUntil.Format("15:04:05"))
			}
			if ctrl.activityRepo != nil {
				_ = ctrl.activityRepo.RecordAuditLog("login_blocked", clientIP, userAgent, "Blocked attempt from banned IP")
			}
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

	result, err := ctrl.authService.Authenticate(req.Username, pwd)
	if err != nil {
		if ctrl.userRepo != nil {
			attempts, isBanned, _ := ctrl.userRepo.RecordFailedLogin(clientIP, 5, 15*time.Minute)
			details := fmt.Sprintf("Failed password for user '%s' (Attempt %d/5)", req.Username, attempts)
			if isBanned {
				details = fmt.Sprintf("IP banned for 15 mins after 5 failed attempts for user '%s'", req.Username)
				if ctrl.activityRepo != nil {
					_ = ctrl.activityRepo.RecordAuditLog("ip_banned", clientIP, userAgent, details)
				}
			} else {
				if ctrl.activityRepo != nil {
					_ = ctrl.activityRepo.RecordAuditLog("login_failed", clientIP, userAgent, details)
				}
			}
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	if ctrl.userRepo != nil {
		_ = ctrl.userRepo.ClearFailedLogin(clientIP)
		if ctrl.activityRepo != nil {
			_ = ctrl.activityRepo.RecordAuditLog("login_success", clientIP, userAgent, fmt.Sprintf("User '%s' authenticated via Emby API", req.Username))
		}
	}

	c.JSON(http.StatusOK, result)
}

// GetUser handles GET /emby/Users/:id
func (ctrl *UserController) GetUser(c *gin.Context) {
	userId := c.Param("id")
	user, ok := ctrl.authService.GetUser(userId)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}

// GetPublicUsers handles GET /emby/Users/Public
func (ctrl *UserController) GetPublicUsers(c *gin.Context) {
	users := ctrl.authService.GetPublicUsers()
	c.JSON(http.StatusOK, users)
}

// GetCurrentItemUserData handles GET /emby/Users/:id/Items/:itemId/UserData
func (ctrl *UserController) GetCurrentItemUserData(c *gin.Context) {
	userId := c.Param("id")
	itemId := c.Param("itemId")

	dto := &model.UserItemDataDto{
		Key: itemId,
	}

	if ctrl.favRepo != nil {
		if isFav, err := ctrl.favRepo.IsFavorite(userId, itemId); err == nil {
			dto.IsFavorite = isFav
		}
	}

	if ctrl.playbackRepo != nil {
		if rec, err := ctrl.playbackRepo.GetProgress(userId, itemId); err == nil && rec != nil {
			dto.PlaybackPositionTicks = rec.PositionTicks
			dto.Played = rec.Played
			dto.PlayCount = rec.PlayCount
			if !rec.LastPlayedDate.IsZero() {
				dto.LastPlayedDate = rec.LastPlayedDate.Format(time.RFC3339)
			}
		}
	}

	c.JSON(http.StatusOK, dto)
}

// MarkPlayedItem handles POST /emby/Users/:id/PlayedItems/:itemId
func (ctrl *UserController) MarkPlayedItem(c *gin.Context) {
	userId := c.Param("id")
	if userId == "" {
		userId = c.Param("userId")
	}
	itemId := c.Param("itemId")

	if ctrl.playbackRepo != nil {
		_ = ctrl.playbackRepo.MarkPlayed(userId, itemId, true)
	}

	ctrl.GetCurrentItemUserData(c)
}

// UnmarkPlayedItem handles DELETE /emby/Users/:id/PlayedItems/:itemId
func (ctrl *UserController) UnmarkPlayedItem(c *gin.Context) {
	userId := c.Param("id")
	if userId == "" {
		userId = c.Param("userId")
	}
	itemId := c.Param("itemId")

	if ctrl.playbackRepo != nil {
		_ = ctrl.playbackRepo.MarkPlayed(userId, itemId, false)
	}

	ctrl.GetCurrentItemUserData(c)
}

// MarkFavoriteItem handles POST /emby/Users/:id/FavoriteItems/:itemId
func (ctrl *UserController) MarkFavoriteItem(c *gin.Context) {
	userId := c.Param("id")
	if userId == "" {
		userId = c.GetString("userId")
	}
	if userId == "" {
		userId = "admin"
	}
	itemId := c.Param("itemId")

	if ctrl.favRepo != nil {
		_ = ctrl.favRepo.MarkFavorite(userId, itemId, true)
		// Cascade favorite to parent series if episode/season
		if strings.HasPrefix(itemId, "bgm_ep_") || strings.HasPrefix(itemId, "bgm_season_") {
			subId := mapper.ExtractSubjectId(itemId)
			if subId > 0 {
				_ = ctrl.favRepo.MarkFavorite(userId, fmt.Sprintf("bgm_sub_%d", subId), true)
			}
		}
	}

	ctrl.GetCurrentItemUserData(c)
}

// UnmarkFavoriteItem handles DELETE /emby/Users/:id/FavoriteItems/:itemId
func (ctrl *UserController) UnmarkFavoriteItem(c *gin.Context) {
	userId := c.Param("id")
	if userId == "" {
		userId = c.GetString("userId")
	}
	if userId == "" {
		userId = "admin"
	}
	itemId := c.Param("itemId")

	if ctrl.favRepo != nil {
		_ = ctrl.favRepo.MarkFavorite(userId, itemId, false)
		if strings.HasPrefix(itemId, "bgm_sub_") {
			subId := mapper.ExtractSubjectId(itemId)
			if subId > 0 {
				_ = ctrl.favRepo.UnmarkAllSubjectFavorites(userId, subId)
			}
		}
	}

	ctrl.GetCurrentItemUserData(c)
}
