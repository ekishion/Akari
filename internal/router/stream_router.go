package router

import (
	"github.com/gin-gonic/gin"
	"akari-bridge/internal/controller/stream"
)

type StreamRouter struct {
	proxyCtrl *stream.ProxyController
}

func NewStreamRouter(proxyCtrl *stream.ProxyController) *StreamRouter {
	return &StreamRouter{
		proxyCtrl: proxyCtrl,
	}
}

func (r *StreamRouter) RegisterRoutes(engine *gin.Engine) {
	engine.GET("/stream/m3u8", r.proxyCtrl.HandleM3U8)
	engine.HEAD("/stream/m3u8", r.proxyCtrl.HandleM3U8)
	engine.GET("/stream/segment", r.proxyCtrl.HandleSegment)
	engine.HEAD("/stream/segment", r.proxyCtrl.HandleSegment)
	engine.GET("/stream/bilibili/mux", r.proxyCtrl.HandleBilibiliMux)
	engine.HEAD("/stream/bilibili/mux", r.proxyCtrl.HandleBilibiliMux)
}
