package embyapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"akari-bridge/internal/auth"
	"akari-bridge/internal/bangumi"
	"akari-bridge/internal/config"
	"akari-bridge/internal/danmaku"
	"akari-bridge/internal/engine"
	"akari-bridge/internal/proxy"
	"akari-bridge/internal/resolver"
	"akari-bridge/internal/rules"
	"akari-bridge/internal/storage"
	"akari-bridge/web"
)

func SetupRouter(
	cfg *config.Config,
	authSvc *auth.AuthService,
	bgmClient *bangumi.Client,
	ruleMgr *rules.RuleManager,
	eng *engine.Engine,
	res *resolver.StreamResolver,
	streamProxy *proxy.StreamProxy,
	danmakuClient *danmaku.Client,
	db *storage.DB,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	// 1. Global CORS Middleware
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, HEAD")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Emby-Token, X-Emby-Authorization, X-MediaBrowser-Token")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Range")
		c.Header("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	sysHandler := NewSystemHandler(cfg)
	usersHandler := NewUsersHandler(authSvc, db)
	viewsHandler := NewViewsHandler(cfg)
	itemsHandler := NewItemsHandler(cfg, bgmClient, authSvc, db)
	playbackHandler := NewPlaybackHandler(cfg, bgmClient, ruleMgr, eng, res, danmakuClient, db)
	adminHandler := NewAdminHandler(cfg, authSvc, ruleMgr, eng, bgmClient, playbackHandler)
	wsHandler := NewWebSocketHandler()

	// WebSocket Endpoints
	r.GET("/embywebsocket", wsHandler.HandleWebSocket)
	r.GET("/websocket", wsHandler.HandleWebSocket)

	// Stream Proxy Endpoints (No Auth required for media players)
	r.GET("/stream/m3u8", streamProxy.HandleM3U8)
	r.HEAD("/stream/m3u8", streamProxy.HandleM3U8)
	r.GET("/stream/segment", streamProxy.HandleSegment)
	r.HEAD("/stream/segment", streamProxy.HandleSegment)

	// Auth Middleware
	authMiddleware := authSvc.Middleware()

	// 2. Admin Web APIs (/api/...)
	apiGroup := r.Group("/api")
	{
		// Public Auth
		apiGroup.POST("/auth/login", adminHandler.Login)
		apiGroup.GET("/auth/me", authMiddleware, adminHandler.GetMe)
		apiGroup.POST("/auth/change-password", authMiddleware, adminHandler.ChangePassword)

		// System
		apiGroup.GET("/system/status", adminHandler.GetStatus)
		apiGroup.GET("/system/config", adminHandler.GetConfig)
		apiGroup.POST("/system/clean-cache", adminHandler.CleanCache)

		// Users
		apiGroup.GET("/users", adminHandler.ListUsers)
		apiGroup.POST("/users", adminHandler.CreateUser)
		apiGroup.DELETE("/users/:id", adminHandler.DeleteUser)
		apiGroup.GET("/users/:id/tokens", adminHandler.GetUserTokens)
		apiGroup.POST("/users/:id/tokens", adminHandler.CreateUserToken)
		apiGroup.DELETE("/users/:id/tokens/:token", adminHandler.RevokeUserToken)
		apiGroup.POST("/users/:id/bangumi", adminHandler.BindUserBangumi)
		apiGroup.DELETE("/users/:id/bangumi", adminHandler.UnbindUserBangumi)

		// Rules
		apiGroup.GET("/rules", adminHandler.ListRules)
		apiGroup.POST("/rules", adminHandler.SaveRule)
		apiGroup.PUT("/rules/:name/toggle", adminHandler.ToggleRule)
		apiGroup.DELETE("/rules/:name", adminHandler.DeleteRule)
		apiGroup.POST("/rules/import-url", adminHandler.ImportRulesFromURL)
		apiGroup.POST("/rules/test", adminHandler.TestRule)

		// History
		apiGroup.GET("/history", adminHandler.ListHistory)
	}

	// 3. Embedded Web SPA (/web)
	webFS, err := web.GetFS()
	if err == nil {
		fileServer := http.FileServer(http.FS(webFS))
		r.GET("/web/*filepath", func(c *gin.Context) {
			p := c.Param("filepath")
			if p == "" || p == "/" {
				c.Request.URL.Path = "/"
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
			f, err := webFS.Open(strings.TrimPrefix(p, "/"))
			if err == nil {
				_ = f.Close()
				c.Request.URL.Path = p
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
			// Fallback to index.html for SPA client-side routes
			c.Request.URL.Path = "/"
			fileServer.ServeHTTP(c.Writer, c.Request)
		})
		r.GET("/web", func(c *gin.Context) {
			c.Redirect(http.StatusMovedPermanently, "/web/")
		})
	}

	// 4. Emby API Routes
	registerRoutes := func(rg *gin.RouterGroup) {
		// System endpoints
		rg.GET("/System/Info/Public", sysHandler.GetPublicInfo)
		rg.GET("/System/Info", sysHandler.GetInfo)
		rg.GET("/System/Ping", sysHandler.Ping)
		rg.POST("/System/Ping", sysHandler.Ping)
		rg.GET("/System/Configuration", sysHandler.GetConfiguration)
		rg.GET("/System/Endpoint", sysHandler.GetEndpoint)

		// Users endpoints
		rg.POST("/Users/AuthenticateByName", usersHandler.AuthenticateByName)
		rg.GET("/Users/Public", usersHandler.GetPublicUsers)

		// Images proxy
		rg.GET("/items/images/proxy", itemsHandler.ProxyImage)
		rg.GET("/Items/:id/Images/Primary", itemsHandler.GetPrimaryImage)

		// Metadata endpoints (Infuse / Emby probes)
		rg.GET("/Items/Counts", itemsHandler.GetItemCounts)
		rg.GET("/Studios", itemsHandler.GetStudios)
		rg.GET("/Genres", itemsHandler.GetGenres)
		rg.GET("/Persons", itemsHandler.GetPersons)
		rg.GET("/Persons/:name", itemsHandler.GetPersons)
		rg.GET("/Persons/:name/Images/Primary", itemsHandler.GetPrimaryImage)
		rg.GET("/Collections", itemsHandler.GetCollections)
		rg.GET("/Playlists", itemsHandler.GetPlaylists)
		rg.GET("/DisplayPreferences/:id", itemsHandler.GetDisplayPreferences)
		rg.POST("/DisplayPreferences/:id", itemsHandler.GetDisplayPreferences)
		rg.GET("/LiveTv/Channels", itemsHandler.GetLiveTvChannels)

		// Subtitles endpoints
		rg.GET("/Videos/:id/subtitles/:index/Stream.ass", playbackHandler.GetAssSubtitleStream)
		rg.GET("/Videos/:id/subtitles/:index/Stream.vtt", playbackHandler.GetVttSubtitleStream)

		// Authenticated group
		authed := rg.Group("")
		authed.Use(authMiddleware)
		{
			authed.GET("/Users/:id", usersHandler.GetUser)
			authed.GET("/Users/:id/Views", viewsHandler.GetUserViews)
			authed.GET("/Users/:id/Items", itemsHandler.GetUserItems)
			authed.GET("/Users/:id/Items/Latest", itemsHandler.GetUserItems)
			authed.GET("/Users/:id/Items/Resume", itemsHandler.GetResumeItems)
			authed.GET("/Users/:id/Items/:itemId", itemsHandler.GetItem)
			authed.GET("/Users/:id/Items/:itemId/UserData", usersHandler.GetCurrentItemUserData)
			authed.POST("/Users/:id/PlayedItems/:itemId", usersHandler.MarkPlayedItem)
			authed.DELETE("/Users/:id/PlayedItems/:itemId", usersHandler.UnmarkPlayedItem)
			authed.POST("/Users/:id/FavoriteItems/:itemId", usersHandler.MarkFavoriteItem)
			authed.DELETE("/Users/:id/FavoriteItems/:itemId", usersHandler.UnmarkFavoriteItem)
			authed.POST("/UserFavoriteItems/:itemId", usersHandler.MarkFavoriteItem)
			authed.DELETE("/UserFavoriteItems/:itemId", usersHandler.UnmarkFavoriteItem)

			// Items & Shows
			authed.GET("/Items", itemsHandler.GetUserItems)
			authed.GET("/Items/:id", itemsHandler.GetItem)
			authed.GET("/Items/:id/ThemeMedia", itemsHandler.GetThemeMedia)
			authed.GET("/Items/:id/ThemeSongs", itemsHandler.GetThemeMedia)
			authed.GET("/Items/:id/Similar", itemsHandler.GetSimilarItems)
			authed.GET("/Shows/:id/Seasons", itemsHandler.GetSeasons)
			authed.GET("/Shows/:id/Episodes", itemsHandler.GetEpisodes)
			authed.GET("/Shows/NextUp", itemsHandler.GetNextUp)

			// Playback & Sessions
			authed.POST("/Items/:id/PlaybackInfo", playbackHandler.GetPlaybackInfo)
			authed.POST("/Sessions/Playing", playbackHandler.ReportPlaybackPlaying)
			authed.POST("/Sessions/Playing/Progress", playbackHandler.ReportPlaybackProgress)
			authed.POST("/Sessions/Playing/Stopped", playbackHandler.ReportPlaybackStopped)
			authed.POST("/Sessions/Capabilities", playbackHandler.SessionCapabilities)
			authed.POST("/Sessions/Capabilities/Full", playbackHandler.SessionCapabilities)
			authed.POST("/Sessions/Playing/Ping", playbackHandler.SessionPing)
			authed.GET("/Sessions/Playing/Ping", playbackHandler.SessionPing)
		}
	}

	// Mount on /emby (standard) and root fallback
	embyGroup := r.Group("/emby")
	registerRoutes(embyGroup)

	rootGroup := r.Group("")
	registerRoutes(rootGroup)

	// Browser visiting root / redirects to /web/
	r.GET("/", func(c *gin.Context) {
		accept := c.GetHeader("Accept")
		if strings.Contains(accept, "text/html") {
			c.Redirect(http.StatusTemporaryRedirect, "/web/")
			return
		}
		c.JSON(http.StatusOK, gin.H{"ServerName": cfg.ServerName, "Version": "1.0.0"})
	})

	return r
}
