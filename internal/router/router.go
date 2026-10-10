package router

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"akari-bridge/internal/config"
	"akari-bridge/internal/controller/emby"
	"akari-bridge/web"
)

type Router struct {
	cfg          *config.Config
	embyRouter   *EmbyRouter
	adminRouter  *AdminRouter
	streamRouter *StreamRouter
	sysCtrl      *emby.SystemController
}

func NewRouter(
	cfg *config.Config,
	embyRouter *EmbyRouter,
	adminRouter *AdminRouter,
	streamRouter *StreamRouter,
	sysCtrl *emby.SystemController,
) *Router {
	return &Router{
		cfg:          cfg,
		embyRouter:   embyRouter,
		adminRouter:  adminRouter,
		streamRouter: streamRouter,
		sysCtrl:      sysCtrl,
	}
}

func (rt *Router) InitEngine() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	// 1. Global Security Headers & CORS Middleware
	r.Use(SecurityHeadersMiddleware())

	// 2. WebSocket Endpoints
	r.GET("/embywebsocket", rt.sysCtrl.HandleWebSocket)
	r.GET("/websocket", rt.sysCtrl.HandleWebSocket)

	// 3. Stream Proxy Endpoints
	rt.streamRouter.RegisterRoutes(r)

	// 4. Admin Web APIs (/api)
	apiGroup := r.Group("/api")
	rt.adminRouter.RegisterRoutes(apiGroup)

	// 5. Embedded Web Dashboard SPA (/web)
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

	// 6. Emby API Routes (Mounted on /emby and root fallback)
	embyGroup := r.Group("/emby")
	rt.embyRouter.RegisterRoutes(embyGroup)

	rootGroup := r.Group("")
	rt.embyRouter.RegisterRoutes(rootGroup)

	// Browser visiting root / redirects to /web/
	r.GET("/", func(c *gin.Context) {
		accept := c.GetHeader("Accept")
		if strings.Contains(accept, "text/html") {
			c.Redirect(http.StatusTemporaryRedirect, "/web/")
			return
		}
		c.JSON(http.StatusOK, gin.H{"ServerName": rt.cfg.ServerName, "Version": "1.0.0"})
	})

	return r
}
