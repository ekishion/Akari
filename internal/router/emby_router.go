package router

import (
	"github.com/gin-gonic/gin"
	"akari-bridge/internal/auth"
	"akari-bridge/internal/controller/emby"
)

type EmbyRouter struct {
	authSvc      *auth.AuthService
	itemCtrl     *emby.ItemController
	playbackCtrl *emby.PlaybackController
	userCtrl     *emby.UserController
	sysCtrl      *emby.SystemController
}

func NewEmbyRouter(
	authSvc *auth.AuthService,
	itemCtrl *emby.ItemController,
	playbackCtrl *emby.PlaybackController,
	userCtrl *emby.UserController,
	sysCtrl *emby.SystemController,
) *EmbyRouter {
	return &EmbyRouter{
		authSvc:      authSvc,
		itemCtrl:     itemCtrl,
		playbackCtrl: playbackCtrl,
		userCtrl:     userCtrl,
		sysCtrl:      sysCtrl,
	}
}

func (r *EmbyRouter) RegisterRoutes(rg *gin.RouterGroup) {
	// System endpoints
	rg.GET("/System/Info/Public", r.sysCtrl.GetPublicInfo)
	rg.GET("/System/Info", r.sysCtrl.GetInfo)
	rg.GET("/System/Ping", r.sysCtrl.Ping)
	rg.POST("/System/Ping", r.sysCtrl.Ping)
	rg.GET("/System/Configuration", r.sysCtrl.GetConfiguration)
	rg.GET("/System/Endpoint", r.sysCtrl.GetEndpoint)

	// Users endpoints
	rg.POST("/Users/AuthenticateByName", r.userCtrl.AuthenticateByName)
	rg.GET("/Users/Public", r.userCtrl.GetPublicUsers)

	// Images proxy
	rg.GET("/items/images/proxy", r.itemCtrl.ProxyImage)
	rg.GET("/Items/:id/Images/Primary", r.itemCtrl.GetPrimaryImage)

	// Metadata endpoints (Infuse / Emby probes)
	rg.GET("/Library/MediaFolders", r.sysCtrl.GetMediaFolders)
	rg.GET("/Items/Root", r.sysCtrl.GetRootFolder)
	rg.GET("/Items/Counts", r.itemCtrl.GetItemCounts)
	rg.GET("/Studios", r.itemCtrl.GetStudios)
	rg.GET("/Genres", r.itemCtrl.GetGenres)
	rg.GET("/Persons", r.itemCtrl.GetPersons)
	rg.GET("/Persons/:name", r.itemCtrl.GetPersons)
	rg.GET("/Persons/:name/Images/Primary", r.itemCtrl.GetPrimaryImage)
	rg.GET("/Collections", r.itemCtrl.GetCollections)
	rg.GET("/Playlists", r.itemCtrl.GetPlaylists)
	rg.GET("/DisplayPreferences/:id", r.itemCtrl.GetDisplayPreferences)
	rg.POST("/DisplayPreferences/:id", r.itemCtrl.GetDisplayPreferences)
	rg.GET("/LiveTv/Channels", r.itemCtrl.GetLiveTvChannels)

	// Subtitles endpoints
	rg.GET("/Videos/:id/subtitles/:index/Stream.ass", r.playbackCtrl.GetAssSubtitleStream)
	rg.GET("/Videos/:id/subtitles/:index/Stream.vtt", r.playbackCtrl.GetVttSubtitleStream)

	// Authenticated Emby group
	authed := rg.Group("")
	authed.Use(r.authSvc.Middleware())
	{
		authed.GET("/Users/:id", r.userCtrl.GetUser)
		authed.GET("/Users/:id/Views", r.sysCtrl.GetUserViews)
		authed.GET("/Users/:id/UserViews", r.sysCtrl.GetUserViews)
		authed.GET("/UserViews", r.sysCtrl.GetUserViews)
		authed.GET("/Views", r.sysCtrl.GetUserViews)
		authed.GET("/Users/:id/GroupingOptions", r.sysCtrl.GetGroupingOptions)
		authed.GET("/Users/:id/Items/Root", r.sysCtrl.GetRootFolder)
		authed.GET("/Users/:id/Items", r.itemCtrl.GetUserItems)
		authed.GET("/Users/:id/Items/Latest", r.itemCtrl.GetLatestItems)
		authed.GET("/Users/:id/Items/Resume", r.itemCtrl.GetResumeItems)
		authed.GET("/Users/:id/Items/:itemId", r.itemCtrl.GetItem)
		authed.GET("/Users/:id/Items/:itemId/UserData", r.userCtrl.GetCurrentItemUserData)
		authed.GET("/Users/:id/Shows/NextUp", r.itemCtrl.GetNextUp)
		authed.POST("/Users/:id/PlayedItems/:itemId", r.userCtrl.MarkPlayedItem)
		authed.DELETE("/Users/:id/PlayedItems/:itemId", r.userCtrl.UnmarkPlayedItem)
		authed.POST("/Users/:id/FavoriteItems/:itemId", r.userCtrl.MarkFavoriteItem)
		authed.DELETE("/Users/:id/FavoriteItems/:itemId", r.userCtrl.UnmarkFavoriteItem)
		authed.POST("/UserFavoriteItems/:itemId", r.userCtrl.MarkFavoriteItem)
		authed.DELETE("/UserFavoriteItems/:itemId", r.userCtrl.UnmarkFavoriteItem)

		// Items & Shows
		authed.GET("/Items", r.itemCtrl.GetUserItems)
		authed.GET("/Items/Latest", r.itemCtrl.GetLatestItems)
		authed.GET("/Items/:id", r.itemCtrl.GetItem)
		authed.GET("/Items/:id/ThemeMedia", r.itemCtrl.GetThemeMedia)
		authed.GET("/Items/:id/ThemeSongs", r.itemCtrl.GetThemeMedia)
		authed.GET("/Items/:id/Similar", r.itemCtrl.GetSimilarItems)
		authed.GET("/Shows/:id/Seasons", r.itemCtrl.GetSeasons)
		authed.GET("/Shows/:id/Episodes", r.itemCtrl.GetEpisodes)
		authed.GET("/Shows/NextUp", r.itemCtrl.GetNextUp)
		authed.GET("/Shows/NextUp/Episodes", r.itemCtrl.GetNextUp)

		// Playback & Sessions
		authed.POST("/Items/:id/PlaybackInfo", r.playbackCtrl.GetPlaybackInfo)
		authed.POST("/Sessions/Playing", r.playbackCtrl.ReportPlaying)
		authed.POST("/Sessions/Playing/Progress", r.playbackCtrl.ReportProgress)
		authed.POST("/Sessions/Playing/Stopped", r.playbackCtrl.ReportStopped)
		authed.POST("/Sessions/Capabilities", r.playbackCtrl.SessionCapabilities)
		authed.POST("/Sessions/Capabilities/Full", r.playbackCtrl.SessionCapabilities)
		authed.POST("/Sessions/Playing/Ping", r.playbackCtrl.SessionPing)
		authed.GET("/Sessions/Playing/Ping", r.playbackCtrl.SessionPing)
	}
}
