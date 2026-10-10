package admin

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"akari-bridge/internal/auth"
	"akari-bridge/internal/bangumi"
	"akari-bridge/internal/domain"
)

type UserController struct {
	authSvc   *auth.AuthService
	bgmClient *bangumi.Client
	userRepo  domain.IUserRepository
}

func NewUserController(authSvc *auth.AuthService, bgmClient *bangumi.Client, userRepo domain.IUserRepository) *UserController {
	return &UserController{
		authSvc:   authSvc,
		bgmClient: bgmClient,
		userRepo:  userRepo,
	}
}

func (ctrl *UserController) ListUsers(c *gin.Context) {
	users := ctrl.authSvc.ListUsers()
	type UserView struct {
		ID            string     `json:"id"`
		Name          string     `json:"name"`
		IsAdmin       bool       `json:"isAdmin"`
		IsDisabled    bool       `json:"isDisabled"`
		HasPassword   bool       `json:"hasPassword"`
		BgmUserID     string     `json:"bgmUserId"`
		HasBgmToken   bool       `json:"hasBgmToken"`
		LastLoginDate *time.Time `json:"lastLoginDate"`
		TokenCount    int        `json:"tokenCount"`
	}

	result := make([]UserView, 0, len(users))
	for _, u := range users {
		tokenCount := 0
		tokens, err := ctrl.authSvc.GetUserTokens(u.Id)
		if err == nil {
			tokenCount = len(tokens)
		}
		bgmToken, bgmUid := ctrl.authSvc.GetUserBangumi(u.Id)
		result = append(result, UserView{
			ID:            u.Id,
			Name:          u.Name,
			IsAdmin:       u.Policy.IsAdministrator,
			IsDisabled:    u.Policy.IsDisabled,
			HasPassword:   u.HasPassword,
			BgmUserID:     bgmUid,
			HasBgmToken:   bgmToken != "",
			LastLoginDate: u.LastLoginDate,
			TokenCount:    tokenCount,
		})
	}

	c.JSON(http.StatusOK, result)
}

type CreateUserReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	IsAdmin  bool   `json:"isAdmin"`
}

func (ctrl *UserController) CreateUser(c *gin.Context) {
	var req CreateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	u, err := ctrl.authSvc.CreateUser(req.Username, req.Password, req.IsAdmin)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, u)
}

func (ctrl *UserController) DeleteUser(c *gin.Context) {
	userId := c.Param("id")
	if err := ctrl.authSvc.DeleteUser(userId); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (ctrl *UserController) SetUserPassword(c *gin.Context) {
	userId := c.Param("id")
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	if err := ctrl.authSvc.UpdatePassword(userId, req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Password updated successfully"})
}

func (ctrl *UserController) GetUserTokens(c *gin.Context) {
	userId := c.Param("id")
	tokens, err := ctrl.authSvc.GetUserTokens(userId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tokens)
}

type CreateTokenReq struct {
	ClientName string `json:"clientName"`
}

func (ctrl *UserController) CreateUserToken(c *gin.Context) {
	userId := c.Param("id")
	var req CreateTokenReq
	_ = c.ShouldBindJSON(&req)
	if req.ClientName == "" {
		req.ClientName = "Web Dashboard"
	}

	token, err := ctrl.authSvc.CreateTokenForUser(userId, req.ClientName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":      token,
		"userId":     userId,
		"clientName": req.ClientName,
	})
}

func (ctrl *UserController) RevokeUserToken(c *gin.Context) {
	token := c.Param("token")
	if err := ctrl.authSvc.RevokeToken(token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

type BangumiBindReq struct {
	AccessToken string `json:"accessToken"`
}

func (ctrl *UserController) BindUserBangumi(c *gin.Context) {
	userId := c.Param("id")
	var req BangumiBindReq
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.AccessToken) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Access token is required"})
		return
	}

	req.AccessToken = strings.TrimSpace(req.AccessToken)
	profile, err := ctrl.bgmClient.GetMe(req.AccessToken)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bangumi validation failed: " + err.Error()})
		return
	}

	if err := ctrl.authSvc.UpdateUserBangumi(userId, req.AccessToken, profile.Username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"username": profile.Username,
		"nickname": profile.Nickname,
		"avatar":   profile.Avatar,
	})
}

func (ctrl *UserController) UnbindUserBangumi(c *gin.Context) {
	userId := c.Param("id")
	if err := ctrl.authSvc.UpdateUserBangumi(userId, "", ""); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
