package emby

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"akari-bridge/internal/bangumi"
	"akari-bridge/internal/config"
	"akari-bridge/internal/danmaku"
	"akari-bridge/internal/domain"
	"akari-bridge/internal/mapper"
	"akari-bridge/internal/model"
)

type PlaybackProgressPayload struct {
	ItemId        string `json:"ItemId"`
	PositionTicks int64  `json:"PositionTicks"`
	TotalTicks    int64  `json:"TotalTicks"`
	Event         string `json:"Event"`
	IsPaused      bool   `json:"IsPaused"`
}

type PlaybackController struct {
	cfg           *config.Config
	playbackSvc   domain.IPlaybackService
	danmakuClient *danmaku.Client
	bangumiClient *bangumi.Client
}

func NewPlaybackController(
	cfg *config.Config,
	playbackSvc domain.IPlaybackService,
	danmakuClient *danmaku.Client,
	bgmClient *bangumi.Client,
) *PlaybackController {
	return &PlaybackController{
		cfg:           cfg,
		playbackSvc:   playbackSvc,
		danmakuClient: danmakuClient,
		bangumiClient: bgmClient,
	}
}

func (ctrl *PlaybackController) GetPlaybackInfo(c *gin.Context) {
	itemId := c.Param("id")
	if itemId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing item id"})
		return
	}

	res, err := ctrl.playbackSvc.ResolvePlayback(c.Request.Context(), itemId)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	proxyBase := ctrl.cfg.GetBaseURLFromRequest(c.Request)
	mediaSources, err := ctrl.playbackSvc.AssembleMediaSources(itemId, proxyBase, res)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := model.PlaybackInfoResponse{
		PlaySessionId: "session_" + itemId,
		MediaSources:  mediaSources,
	}
	c.JSON(http.StatusOK, resp)
}

func (ctrl *PlaybackController) ReportPlaying(c *gin.Context) {
	var req PlaybackProgressPayload
	_ = c.ShouldBindJSON(&req)
	userId := c.GetString("userId")
	if req.ItemId != "" {
		_ = ctrl.playbackSvc.ReportProgress(userId, req.ItemId, req.PositionTicks, req.TotalTicks, req.IsPaused)
	}
	c.Status(http.StatusNoContent)
}

func (ctrl *PlaybackController) ReportProgress(c *gin.Context) {
	var req PlaybackProgressPayload
	_ = c.ShouldBindJSON(&req)
	userId := c.GetString("userId")
	if req.ItemId != "" {
		_ = ctrl.playbackSvc.ReportProgress(userId, req.ItemId, req.PositionTicks, req.TotalTicks, req.IsPaused)
	}
	c.Status(http.StatusNoContent)
}

func (ctrl *PlaybackController) ReportStopped(c *gin.Context) {
	var req PlaybackProgressPayload
	_ = c.ShouldBindJSON(&req)
	userId := c.GetString("userId")
	if req.ItemId != "" {
		_ = ctrl.playbackSvc.ReportStopped(userId, req.ItemId, req.PositionTicks)
	}
	c.Status(http.StatusNoContent)
}

func (ctrl *PlaybackController) GetAssSubtitleStream(c *gin.Context) {
	itemId := c.Param("id")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	title, epIndex := ctrl.extractTitleAndEp(itemId)

	var comments []danmaku.DanmakuComment
	var err error

	subId, _ := mapper.ExtractEpisodeInfo(itemId)
	if subId == 0 && strings.HasPrefix(itemId, "bgm_ep_") {
		clean := strings.TrimPrefix(itemId, "bgm_ep_")
		if epId, err := strconv.Atoi(clean); err == nil && ctrl.bangumiClient != nil {
			_, realSubId, err := ctrl.bangumiClient.GetEpisode(epId)
			if err == nil {
				subId = realSubId
			}
		}
	}

	if subId > 0 && ctrl.danmakuClient != nil {
		comments, err = ctrl.danmakuClient.GetCommentsByBgmId(ctx, subId, epIndex)
	}

	if (err != nil || len(comments) == 0) && title != "" && ctrl.danmakuClient != nil {
		comments, _ = ctrl.danmakuClient.SearchAndGetComments(ctx, title, epIndex)
	}

	assContent := danmaku.CommentsToAss(comments, title)

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Cache-Control", "public, max-age=3600")
	c.String(http.StatusOK, assContent)
}

func (ctrl *PlaybackController) GetVttSubtitleStream(c *gin.Context) {
	itemId := c.Param("id")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	title, epIndex := ctrl.extractTitleAndEp(itemId)

	var comments []danmaku.DanmakuComment
	subId, _ := mapper.ExtractEpisodeInfo(itemId)
	if subId > 0 && ctrl.danmakuClient != nil {
		comments, _ = ctrl.danmakuClient.GetCommentsByBgmId(ctx, subId, epIndex)
	}
	if len(comments) == 0 && title != "" && ctrl.danmakuClient != nil {
		comments, _ = ctrl.danmakuClient.SearchAndGetComments(ctx, title, epIndex)
	}

	var sb strings.Builder
	sb.WriteString("WEBVTT\n\n")
	for i, cmt := range comments {
		start := time.Duration(cmt.Time * float64(time.Second))
		end := start + 4*time.Second
		startStr := fmt.Sprintf("%02d:%02d:%02d.%03d", int(start.Hours()), int(start.Minutes())%60, int(start.Seconds())%60, start.Milliseconds()%1000)
		endStr := fmt.Sprintf("%02d:%02d:%02d.%03d", int(end.Hours()), int(end.Minutes())%60, int(end.Seconds())%60, end.Milliseconds()%1000)
		sb.WriteString(fmt.Sprintf("%d\n%s --> %s\n%s\n\n", i+1, startStr, endStr, cmt.Text))
	}

	c.Header("Content-Type", "text/vtt; charset=utf-8")
	c.Header("Cache-Control", "public, max-age=3600")
	c.String(http.StatusOK, sb.String())
}

func (ctrl *PlaybackController) SessionCapabilities(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func (ctrl *PlaybackController) SessionPing(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func (ctrl *PlaybackController) extractTitleAndEp(itemId string) (string, int) {
	if strings.HasPrefix(itemId, "bgm_ep_") {
		clean := strings.TrimPrefix(itemId, "bgm_ep_")
		parts := strings.Split(clean, "_")
		if len(parts) >= 2 {
			subId, _ := strconv.Atoi(parts[0])
			epIndex, _ := strconv.Atoi(parts[1])
			if epIndex <= 0 {
				epIndex = 1
			}
			if subId > 0 && ctrl.bangumiClient != nil {
				if sub, err := ctrl.bangumiClient.GetSubject(subId); err == nil && sub != nil {
					return sub.GetDisplayName(), epIndex
				}
			}
			return "", epIndex
		}
	}
	return "", 1
}
