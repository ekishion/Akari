package stream

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"akari-bridge/internal/bilibili"
	"akari-bridge/internal/proxy"
)

type ProxyController struct {
	streamProxy *proxy.StreamProxy
	dashMuxer   *bilibili.DASHMuxer
	biliAuth    *bilibili.AuthManager
}

func NewProxyController(streamProxy *proxy.StreamProxy, dashMuxer *bilibili.DASHMuxer, biliAuth *bilibili.AuthManager) *ProxyController {
	return &ProxyController{
		streamProxy: streamProxy,
		dashMuxer:   dashMuxer,
		biliAuth:    bilibiliAuth(biliAuth),
	}
}

func bilibiliAuth(auth *bilibili.AuthManager) *bilibili.AuthManager {
	return auth
}

func (ctrl *ProxyController) HandleM3U8(c *gin.Context) {
	if ctrl.streamProxy != nil {
		ctrl.streamProxy.HandleM3U8(c)
	}
}

func (ctrl *ProxyController) HandleSegment(c *gin.Context) {
	if ctrl.streamProxy != nil {
		ctrl.streamProxy.HandleSegment(c)
	}
}

func (ctrl *ProxyController) HandleBilibiliMux(c *gin.Context) {
	if ctrl.dashMuxer == nil || !ctrl.dashMuxer.IsAvailable() {
		c.String(http.StatusServiceUnavailable, "ffmpeg is not available on this server for DASH live muxing")
		return
	}

	if c.Request.Method == http.MethodHead {
		c.Header("Content-Type", "video/mp4")
		c.Header("Accept-Ranges", "none")
		c.Status(http.StatusOK)
		return
	}

	videoURL := bilibili.SanitizeUposURL(c.Query("video_url"), nil)
	audioURL := bilibili.SanitizeUposURL(c.Query("audio_url"), nil)
	referer := c.Query("referer")
	ua := c.Query("ua")

	if videoURL == "" {
		c.String(http.StatusBadRequest, "Missing video_url parameter")
		return
	}

	var cookie string
	if ctrl.biliAuth != nil {
		creds := ctrl.biliAuth.GetCredentials()
		if creds != nil && creds.SessData != "" {
			cookie = fmt.Sprintf("SESSDATA=%s", creds.SessData)
		}
	}

	var startSeconds float64
	if startStr := c.Query("start"); startStr != "" {
		startSeconds, _ = strconv.ParseFloat(startStr, 64)
	}

	c.Header("Content-Type", "video/mp4")
	c.Header("Cache-Control", "no-cache, no-store")
	c.Header("Connection", "keep-alive")
	c.Header("Accept-Ranges", "none")

	c.Status(http.StatusOK)
	c.Writer.Flush()

	if err := ctrl.dashMuxer.MuxStream(c.Request.Context(), c.Writer, videoURL, audioURL, referer, ua, cookie, startSeconds); err != nil {
		log.Printf("[StreamProxy] DASH mux stream finished: %v", err)
	}
}
