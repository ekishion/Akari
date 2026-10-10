package admin

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"akari-bridge/internal/bilibili"
)

type BilibiliController struct {
	biliAuth *bilibili.AuthManager
}

func NewBilibiliController(biliAuth *bilibili.AuthManager) *BilibiliController {
	return &BilibiliController{
		biliAuth: biliAuth,
	}
}

func (ctrl *BilibiliController) GetBilibiliStatus(c *gin.Context) {
	if ctrl.biliAuth == nil {
		c.JSON(http.StatusOK, gin.H{"is_login": false, "enabled": false})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	st := ctrl.biliAuth.GetStatus(ctx)
	c.JSON(http.StatusOK, st)
}

func (ctrl *BilibiliController) UpdateBilibiliConfig(c *gin.Context) {
	if ctrl.biliAuth == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Bilibili auth not initialized"})
		return
	}

	var req struct {
		SessData       *string `json:"sessdata"`
		BiliJct        *string `json:"bili_jct"`
		Buvid3         *string `json:"buvid3"`
		DedeUserID     *string `json:"dede_user_id"`
		Enabled        *bool   `json:"enabled"`
		PreferBilibili *bool   `json:"prefer_bilibili"`
		MaxQuality     *int    `json:"max_quality"`
		StreamMode     *string `json:"stream_mode"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update credentials if provided
	if req.SessData != nil || req.Buvid3 != nil {
		currentCreds := ctrl.biliAuth.GetCredentials()
		if currentCreds == nil {
			currentCreds = &bilibili.BilibiliCredentials{}
		}
		if req.SessData != nil {
			currentCreds.SessData = strings.TrimSpace(*req.SessData)
		}
		if req.BiliJct != nil {
			currentCreds.BiliJct = strings.TrimSpace(*req.BiliJct)
		}
		if req.Buvid3 != nil {
			currentCreds.Buvid3 = strings.TrimSpace(*req.Buvid3)
		}
		if req.DedeUserID != nil {
			currentCreds.DedeUserID = strings.TrimSpace(*req.DedeUserID)
		}
		if err := ctrl.biliAuth.SetCredentials(currentCreds); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save credentials: " + err.Error()})
			return
		}
	}

	// Update settings if provided
	currentSettings := ctrl.biliAuth.GetSettings()
	if currentSettings == nil {
		currentSettings = &bilibili.BilibiliSettings{Enabled: true, PreferBilibili: true, MaxQuality: 80, StreamMode: "direct"}
	}
	if req.Enabled != nil {
		currentSettings.Enabled = *req.Enabled
	}
	if req.PreferBilibili != nil {
		currentSettings.PreferBilibili = *req.PreferBilibili
	}
	if req.MaxQuality != nil && *req.MaxQuality > 0 {
		currentSettings.MaxQuality = *req.MaxQuality
	}
	if req.StreamMode != nil && *req.StreamMode != "" {
		currentSettings.StreamMode = *req.StreamMode
	}
	if err := ctrl.biliAuth.SetSettings(currentSettings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save settings: " + err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	st := ctrl.biliAuth.GetStatus(ctx)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"status":  st,
	})
}

func (ctrl *BilibiliController) GenerateBilibiliQR(c *gin.Context) {
	if ctrl.biliAuth == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Bilibili auth not initialized"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	res, err := ctrl.biliAuth.GenerateQR(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (ctrl *BilibiliController) PollBilibiliQR(c *gin.Context) {
	if ctrl.biliAuth == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Bilibili auth not initialized"})
		return
	}

	key := c.Query("qrcode_key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "qrcode_key parameter is required"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	pollResp, isSuccess, err := ctrl.biliAuth.PollQR(ctx, key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"poll":       pollResp,
		"is_success": isSuccess,
	})
}

func (ctrl *BilibiliController) LogoutBilibili(c *gin.Context) {
	if ctrl.biliAuth == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Bilibili auth not initialized"})
		return
	}

	if err := ctrl.biliAuth.ClearCredentials(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
