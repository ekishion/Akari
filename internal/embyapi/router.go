package embyapi

import (
	"io/fs"
	"net/http"
	"path"
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
	adminAuth *auth.AdminAuthService,
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
	_ = r.SetTrustedProxies(nil)
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	playbackHandler := NewPlaybackHandler(cfg, bgmClient, ruleMgr, eng, res, danmakuClient, db)
	adminHandler := NewAdminHandler(cfg, authSvc, ruleMgr, eng, bgmClient, playbackHandler, db)
	adminSecHandler := NewAdminSecurityHandler(cfg, db, adminAuth, authSvc, ruleMgr, playbackHandler)
	sysHandler := NewSystemHandler(cfg)
	usersHandler := NewUsersHandler(authSvc, db)
	viewsHandler := NewViewsHandler(cfg)
	itemsHandler := NewItemsHandler(cfg, bgmClient, authSvc, db)
	wsHandler := NewWebSocketHandler()
	authMiddleware := authSvc.Middleware()

	// 1. Global Security Headers & CORS Middleware
	r.Use(adminSecHandler.SecurityHeadersMiddleware())

	// WebSocket Endpoints
	r.GET("/embywebsocket", wsHandler.HandleWebSocket)
	r.GET("/websocket", wsHandler.HandleWebSocket)

	// Stream Proxy Endpoints (No Auth required for media players)
	r.GET("/stream/m3u8", streamProxy.HandleM3U8)
	r.HEAD("/stream/m3u8", streamProxy.HandleM3U8)
	r.GET("/stream/segment", streamProxy.HandleSegment)
	r.HEAD("/stream/segment", streamProxy.HandleSegment)

	// 2. Admin Web APIs (/api/...)
	apiGroup := r.Group("/api")
	{
		// Public Auth
		apiGroup.POST("/auth/login", adminSecHandler.Login)
		apiGroup.GET("/events", adminSecHandler.EventsStream)

		// Protected Admin Routes
		protected := apiGroup.Group("")
		protected.Use(adminSecHandler.AdminAuthMiddleware())
		{
			// Admin Profile & Password
			protected.POST("/auth/logout", adminSecHandler.Logout)
			protected.GET("/auth/me", adminSecHandler.GetProfile)
			protected.POST("/auth/change-password", adminSecHandler.ChangePassword)

			// Security & Audit
			protected.GET("/security/audit", adminSecHandler.GetAuditLogs)
			protected.DELETE("/security/audit", adminSecHandler.ClearAuditLogs)
			protected.GET("/security/bans", adminSecHandler.GetBannedIPs)
			protected.POST("/security/unban", adminSecHandler.UnbanIP)

			// System
			protected.GET("/system/status", adminHandler.GetStatus)
			protected.GET("/system/config", adminHandler.GetConfig)
			protected.POST("/system/config", adminHandler.UpdateConfig)
			protected.PUT("/system/config", adminHandler.UpdateConfig)
			protected.POST("/system/clean-cache", adminHandler.CleanCache)
			protected.POST("/system/bangumi/test", adminHandler.TestBangumiEndpoint)

			// Users
			protected.GET("/users", adminHandler.ListUsers)
			protected.POST("/users", adminHandler.CreateUser)
			protected.DELETE("/users/:id", adminHandler.DeleteUser)
			protected.POST("/users/:id/password", adminHandler.SetUserPassword)
			protected.GET("/users/:id/tokens", adminHandler.GetUserTokens)
			protected.POST("/users/:id/tokens", adminHandler.CreateUserToken)
			protected.DELETE("/users/:id/tokens/:token", adminHandler.RevokeUserToken)
			protected.POST("/users/:id/bangumi", adminHandler.BindUserBangumi)
			protected.DELETE("/users/:id/bangumi", adminHandler.UnbindUserBangumi)

			// Rules
			protected.GET("/rules", adminHandler.ListRules)
			protected.GET("/rules/:name", adminHandler.GetRule)
			protected.POST("/rules", adminHandler.SaveRule)
			protected.PUT("/rules/:name", adminHandler.SaveRule)
			protected.PUT("/rules/:name/toggle", adminHandler.ToggleRule)
			protected.DELETE("/rules/:name", adminHandler.DeleteRule)
			protected.POST("/rules/import-url", adminHandler.ImportRulesFromURL)
			protected.POST("/rules/update-all", adminHandler.UpdateAllRules)
			protected.POST("/rules/test", adminHandler.TestRule)

			// Aliases & Synonyms
			protected.GET("/aliases/synonyms", adminHandler.ListGlobalSynonyms)
			protected.POST("/aliases/synonyms", adminHandler.UpsertGlobalSynonym)
			protected.DELETE("/aliases/synonyms/:pattern", adminHandler.DeleteGlobalSynonym)
			protected.POST("/aliases/synonyms/reset", adminHandler.ResetGlobalSynonyms)

			protected.GET("/aliases/subjects", adminHandler.ListSubjectAliases)
			protected.POST("/aliases/subjects", adminHandler.UpsertSubjectAliases)
			protected.DELETE("/aliases/subjects/:subject_id", adminHandler.DeleteSubjectAliases)

			// History
			protected.GET("/history", adminHandler.ListHistory)
		}
	}

	// 3. Embedded Web SPA (/web)
	webFS, err := web.GetFS()
	if err == nil {
		fileServer := http.FileServer(http.FS(webFS))
		r.GET("/web/*filepath", func(c *gin.Context) {
			rawPath := c.Param("filepath")
			if rawPath == "" || rawPath == "/" {
				c.Request.URL.Path = "/"
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}

			// Security: Strict path traversal sanitization
			cleanPath := path.Clean(strings.TrimPrefix(rawPath, "/"))
			if strings.HasPrefix(cleanPath, "..") || strings.Contains(cleanPath, "/..") || strings.Contains(cleanPath, "\x00") || !fs.ValidPath(cleanPath) {
				c.Status(http.StatusForbidden)
				return
			}

			f, err := webFS.Open(cleanPath)
			if err == nil {
				_ = f.Close()
				c.Request.URL.Path = "/" + cleanPath
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
		rg.GET("/Library/MediaFolders", viewsHandler.GetMediaFolders)
		rg.GET("/Items/Root", viewsHandler.GetRootFolder)
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
			authed.GET("/Users/:id/UserViews", viewsHandler.GetUserViews)
			authed.GET("/UserViews", viewsHandler.GetUserViews)
			authed.GET("/Views", viewsHandler.GetUserViews)
			authed.GET("/Users/:id/GroupingOptions", viewsHandler.GetGroupingOptions)
			authed.GET("/Users/:id/Items/Root", viewsHandler.GetRootFolder)
			authed.GET("/Users/:id/Items", itemsHandler.GetUserItems)
			authed.GET("/Users/:id/Items/Latest", itemsHandler.GetLatestItems)
			authed.GET("/Users/:id/Items/Resume", itemsHandler.GetResumeItems)
			authed.GET("/Users/:id/Items/:itemId", itemsHandler.GetItem)
			authed.GET("/Users/:id/Items/:itemId/UserData", usersHandler.GetCurrentItemUserData)
			authed.GET("/Users/:id/Shows/NextUp", itemsHandler.GetNextUp)
			authed.POST("/Users/:id/PlayedItems/:itemId", usersHandler.MarkPlayedItem)
			authed.DELETE("/Users/:id/PlayedItems/:itemId", usersHandler.UnmarkPlayedItem)
			authed.POST("/Users/:id/FavoriteItems/:itemId", usersHandler.MarkFavoriteItem)
			authed.DELETE("/Users/:id/FavoriteItems/:itemId", usersHandler.UnmarkFavoriteItem)
			authed.POST("/UserFavoriteItems/:itemId", usersHandler.MarkFavoriteItem)
			authed.DELETE("/UserFavoriteItems/:itemId", usersHandler.UnmarkFavoriteItem)

			// Items & Shows
			authed.GET("/Items", itemsHandler.GetUserItems)
			authed.GET("/Items/Latest", itemsHandler.GetLatestItems)
			authed.GET("/Items/:id", itemsHandler.GetItem)
			authed.GET("/Items/:id/ThemeMedia", itemsHandler.GetThemeMedia)
			authed.GET("/Items/:id/ThemeSongs", itemsHandler.GetThemeMedia)
			authed.GET("/Items/:id/Similar", itemsHandler.GetSimilarItems)
			authed.GET("/Shows/:id/Seasons", itemsHandler.GetSeasons)
			authed.GET("/Shows/:id/Episodes", itemsHandler.GetEpisodes)
			authed.GET("/Shows/NextUp", itemsHandler.GetNextUp)
			authed.GET("/Shows/NextUp/Episodes", itemsHandler.GetNextUp)

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
