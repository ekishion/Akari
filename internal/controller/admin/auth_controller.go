package admin

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"akari-bridge/internal/auth"
	"akari-bridge/internal/domain"
	"akari-bridge/internal/model"
)

type AuthController struct {
	adminAuth    *auth.AdminAuthService
	authSvc      *auth.AuthService
	activityRepo domain.IActivityRepository
}

func NewAuthController(adminAuth *auth.AdminAuthService, authSvc *auth.AuthService, activityRepo domain.IActivityRepository) *AuthController {
	return &AuthController{
		adminAuth:    adminAuth,
		authSvc:      authSvc,
		activityRepo: activityRepo,
	}
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (ctrl *AuthController) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	clientIP := c.ClientIP()
	userAgent := c.Request.UserAgent()

	token, expiresAt, err := ctrl.adminAuth.Login(req.Username, req.Password, clientIP, userAgent)
	if err != nil {
		if strings.Contains(err.Error(), "locked") {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":     token,
		"expiresAt": expiresAt.Format(time.RFC3339),
		"username":  req.Username,
	})
}

func (ctrl *AuthController) Logout(c *gin.Context) {
	username, _ := c.Get("admin_username")
	clientIP := c.ClientIP()
	userAgent := c.Request.UserAgent()

	if ctrl.activityRepo != nil {
		_ = ctrl.activityRepo.RecordAuditLog("logout", clientIP, userAgent, fmt.Sprintf("Admin '%v' logged out", username))
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

func (ctrl *AuthController) GetProfile(c *gin.Context) {
	username, _ := c.Get("admin_username")
	c.JSON(http.StatusOK, gin.H{
		"username": username,
		"role":     "administrator",
	})
}

func (ctrl *AuthController) GetMe(c *gin.Context) {
	val, exists := c.Get("user")
	if !exists || val == nil {
		c.JSON(http.StatusOK, gin.H{"guest": true})
		return
	}
	user := val.(*model.UserDto)
	bgmToken, bgmUid := ctrl.authSvc.GetUserBangumi(user.Id)
	c.JSON(http.StatusOK, gin.H{
		"id":          user.Id,
		"name":        user.Name,
		"isAdmin":     user.Policy.IsAdministrator,
		"hasPassword": user.HasPassword,
		"bgmUserId":   bgmUid,
		"hasBgmToken": bgmToken != "",
	})
}

type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

func (ctrl *AuthController) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters"})
		return
	}

	usernameVal, _ := c.Get("admin_username")
	username, ok := usernameVal.(string)
	if !ok || username == "" {
		username = "admin"
	}

	clientIP := c.ClientIP()
	userAgent := c.Request.UserAgent()

	if err := ctrl.adminAuth.ChangePassword(username, req.OldPassword, req.NewPassword, clientIP, userAgent); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if ctrl.authSvc != nil {
		_ = ctrl.authSvc.UpdatePassword(username, req.NewPassword)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}
