package emby

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"akari-bridge/internal/auth"
	"akari-bridge/internal/bangumi"
	"akari-bridge/internal/config"
	"akari-bridge/internal/domain"
	"akari-bridge/internal/mapper"
	"akari-bridge/internal/model"
)

func generateFallbackPNG(r, g, b uint8, width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	c := color.RGBA{R: r, G: g, B: b, A: 255}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

type ItemController struct {
	cfg           *config.Config
	catalogSvc    domain.ICatalogService
	bangumiClient *bangumi.Client
	authSvc       *auth.AuthService
	playbackRepo  domain.IPlaybackRepository
}

func NewItemController(
	cfg *config.Config,
	catalogSvc domain.ICatalogService,
	bgmClient *bangumi.Client,
	authSvc *auth.AuthService,
	playbackRepo domain.IPlaybackRepository,
) *ItemController {
	return &ItemController{
		cfg:           cfg,
		catalogSvc:    catalogSvc,
		bangumiClient: bgmClient,
		authSvc:       authSvc,
		playbackRepo:  playbackRepo,
	}
}

func (ctrl *ItemController) GetUserItems(c *gin.Context) {
	userId := c.Param("id")
	if userId == "" {
		userId = c.GetString("userId")
	}
	parentId := c.Query("ParentId")
	searchTerm := c.Query("SearchTerm")
	limit, _ := strconv.Atoi(c.DefaultQuery("Limit", "50"))
	startIndex, _ := strconv.Atoi(c.DefaultQuery("StartIndex", "0"))
	if limit <= 0 {
		limit = 50
	}

	filters := make(map[string]string)
	if c.Query("IsFavorite") == "true" || c.Query("IsFavorite") == "1" || strings.Contains(c.Query("Filters"), "IsFavorite") {
		filters["IsFavorite"] = "true"
	}

	res, err := ctrl.catalogSvc.GetItems(c.Request.Context(), userId, parentId, searchTerm, filters, startIndex, limit)
	if err != nil {
		c.JSON(http.StatusOK, model.NewQueryResult([]model.BaseItemDto{}))
		return
	}
	c.JSON(http.StatusOK, res)
}

func (ctrl *ItemController) GetShows(c *gin.Context) {
	userId := c.GetString("userId")
	limit, _ := strconv.Atoi(c.DefaultQuery("Limit", "50"))
	startIndex, _ := strconv.Atoi(c.DefaultQuery("StartIndex", "0"))
	if limit <= 0 {
		limit = 50
	}

	res, err := ctrl.catalogSvc.GetShows(c.Request.Context(), userId, startIndex, limit)
	if err != nil {
		c.JSON(http.StatusOK, model.NewQueryResult([]model.BaseItemDto{}))
		return
	}
	c.JSON(http.StatusOK, res)
}

func (ctrl *ItemController) GetSeasons(c *gin.Context) {
	userId := c.GetString("userId")
	seriesId := c.Param("id")
	res, err := ctrl.catalogSvc.GetSeasons(c.Request.Context(), userId, seriesId)
	if err != nil {
		c.JSON(http.StatusOK, model.NewQueryResult([]model.BaseItemDto{}))
		return
	}
	c.JSON(http.StatusOK, res)
}

func (ctrl *ItemController) GetEpisodes(c *gin.Context) {
	userId := c.GetString("userId")
	seriesId := c.Param("id")
	seasonId := c.Query("SeasonId")
	res, err := ctrl.catalogSvc.GetEpisodes(c.Request.Context(), userId, seriesId, seasonId)
	if err != nil {
		c.JSON(http.StatusOK, model.NewQueryResult([]model.BaseItemDto{}))
		return
	}
	c.JSON(http.StatusOK, res)
}

func (ctrl *ItemController) GetNextUp(c *gin.Context) {
	userId := c.GetString("userId")
	if userId == "" {
		userId = c.Query("UserId")
	}
	seriesId := c.Query("SeriesId")
	res, err := ctrl.catalogSvc.GetNextUp(c.Request.Context(), userId, seriesId)
	if err != nil {
		c.JSON(http.StatusOK, model.NewQueryResult([]model.BaseItemDto{}))
		return
	}
	c.JSON(http.StatusOK, res)
}

func (ctrl *ItemController) GetSimilarItems(c *gin.Context) {
	userId := c.GetString("userId")
	itemId := c.Param("id")
	limit, _ := strconv.Atoi(c.DefaultQuery("Limit", "10"))
	res, err := ctrl.catalogSvc.GetSimilarItems(c.Request.Context(), userId, itemId, limit)
	if err != nil {
		c.JSON(http.StatusOK, model.NewQueryResult([]model.BaseItemDto{}))
		return
	}
	c.JSON(http.StatusOK, res)
}

func (ctrl *ItemController) GetItem(c *gin.Context) {
	userId := c.GetString("userId")
	if userId == "" {
		userId = c.Param("id")
	}
	itemId := c.Param("itemId")
	if itemId == "" {
		itemId = c.Param("id")
	}

	// Check if requesting view folder
	views := mapper.CreateVirtualViews(ctrl.cfg.ServerId)
	for _, v := range views {
		if v.Id == itemId {
			c.JSON(http.StatusOK, v)
			return
		}
	}

	item, err := ctrl.catalogSvc.GetItem(c.Request.Context(), userId, itemId)
	if err != nil || item == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (ctrl *ItemController) GetLatestItems(c *gin.Context) {
	userId := c.Param("id")
	if userId == "" {
		userId = c.GetString("userId")
	}
	parentId := c.Query("ParentId")
	limitStr := c.DefaultQuery("Limit", "16")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 16
	}

	items := make([]model.BaseItemDto, 0)

	switch strings.ToLower(parentId) {
	case mapper.ViewIdSchedule, "schedule":
		subjects, err := ctrl.bangumiClient.GetCalendar()
		if err == nil {
			for _, sub := range subjects {
				item := mapper.SubjectToSeries(&sub, ctrl.cfg.ServerId)
				items = append(items, item)
			}
		}
	case mapper.ViewIdWatching, "watching":
		var userSubjects []bangumi.BangumiSubject
		seenSubject := make(map[int]bool)

		if ctrl.playbackRepo != nil {
			watchingSubIds := ctrl.playbackRepo.GetUserWatchingSubjectIDs(userId, limit+10)
			for _, sid := range watchingSubIds {
				if !seenSubject[sid] {
					if sub, err := ctrl.bangumiClient.GetSubject(sid); err == nil && sub != nil {
						seenSubject[sid] = true
						userSubjects = append(userSubjects, *sub)
					}
				}
			}
		}

		if ctrl.authSvc != nil {
			bgmToken, bgmUid := ctrl.authSvc.GetUserBangumi(userId)
			if bgmUid != "" || bgmToken != "" {
				userTarget := bgmUid
				if userTarget == "" {
					userTarget = "@me"
				}
				if subs, err := ctrl.bangumiClient.GetUserCollections(userTarget, bgmToken, 3, limit, 0); err == nil && len(subs) > 0 {
					for _, sub := range subs {
						if !seenSubject[sub.Id] {
							seenSubject[sub.Id] = true
							userSubjects = append(userSubjects, sub)
						}
					}
				}
			}
		}

		for _, sub := range userSubjects {
			item := mapper.SubjectToSeries(&sub, ctrl.cfg.ServerId)
			items = append(items, item)
		}
	case mapper.ViewIdTrending, "trending", "":
		fallthrough
	default:
		fetchLimit := limit
		if fetchLimit < 20 {
			fetchLimit = 20
		}
		subjects, err := ctrl.bangumiClient.GetTrending(fetchLimit, 0)
		if err == nil {
			for _, sub := range subjects {
				item := mapper.SubjectToSeries(&sub, ctrl.cfg.ServerId)
				items = append(items, item)
			}
		}
	}

	if len(items) > limit {
		items = items[:limit]
	}

	c.JSON(http.StatusOK, items)
}

func (ctrl *ItemController) GetResumeItems(c *gin.Context) {
	userId := c.Param("id")
	if userId == "" {
		userId = c.GetString("userId")
	}
	limitStr := c.DefaultQuery("Limit", "20")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 20
	}

	items := make([]model.BaseItemDto, 0)
	if ctrl.playbackRepo != nil {
		itemIds := ctrl.playbackRepo.GetUserResumeItems(userId, limit)
		for _, id := range itemIds {
			if strings.HasPrefix(id, "bgm_ep_") {
				subId, epIndex := mapper.ExtractEpisodeInfo(id)
				epId := mapper.ExtractEpisodeId(id)
				if subId > 0 {
					sub, err := ctrl.bangumiClient.GetSubject(subId)
					if err == nil && sub != nil {
						var targetEp *bangumi.BangumiEpisode
						if epId > 0 {
							if ep, _, err := ctrl.bangumiClient.GetEpisode(epId); err == nil && ep != nil {
								targetEp = ep
							}
						}
						if targetEp == nil {
							episodes, err := ctrl.bangumiClient.GetEpisodes(subId)
							if err == nil {
								for _, ep := range episodes {
									if ep.Ep == epIndex || int(ep.Sort) == epIndex {
										targetEp = &ep
										break
									}
								}
							}
						}
						if targetEp != nil {
							item := mapper.EpisodeToEmbyEpisode(targetEp, sub, ctrl.cfg.ServerId)
							if totalTicks := ctrl.playbackRepo.GetItemTotalTicks(item.Id); totalTicks > 0 {
								item.RunTimeTicks = totalTicks
							}
							items = append(items, item)
						}
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, model.NewQueryResult(items))
}

func (ctrl *ItemController) GetItemCounts(c *gin.Context) {
	historyCount := 0
	if ctrl.playbackRepo != nil {
		if histories, err := ctrl.playbackRepo.ListRecentHistories(100); err == nil {
			historyCount = len(histories)
		}
	}
	seriesCount := 60
	episodeCount := seriesCount * 12
	c.JSON(http.StatusOK, gin.H{
		"MovieCount":       0,
		"SeriesCount":      seriesCount,
		"EpisodeCount":     episodeCount,
		"GameCount":        0,
		"ArtistCount":      0,
		"ProgramCount":     0,
		"SongCount":        0,
		"AlbumCount":       0,
		"MusicVideoCount":  0,
		"BoxSetCount":      0,
		"BookCount":        0,
		"ItemCount":        seriesCount + episodeCount + historyCount,
	})
}

func (ctrl *ItemController) GetThemeMedia(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"ThemeVideosResult":     model.NewQueryResult([]model.BaseItemDto{}),
		"ThemeSongsResult":      model.NewQueryResult([]model.BaseItemDto{}),
		"SoundtrackSongsResult": model.NewQueryResult([]model.BaseItemDto{}),
	})
}

func (ctrl *ItemController) GetStudios(c *gin.Context) {
	c.JSON(http.StatusOK, model.NewQueryResult([]model.BaseItemDto{}))
}

func (ctrl *ItemController) GetGenres(c *gin.Context) {
	c.JSON(http.StatusOK, model.NewQueryResult([]model.BaseItemDto{}))
}

func (ctrl *ItemController) GetPersons(c *gin.Context) {
	c.JSON(http.StatusOK, model.NewQueryResult([]model.BaseItemDto{}))
}

func (ctrl *ItemController) GetCollections(c *gin.Context) {
	c.JSON(http.StatusOK, model.NewQueryResult([]model.BaseItemDto{}))
}

func (ctrl *ItemController) GetPlaylists(c *gin.Context) {
	c.JSON(http.StatusOK, model.NewQueryResult([]model.BaseItemDto{}))
}

func (ctrl *ItemController) GetDisplayPreferences(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"Id":                      id,
		"ViewType":                "Thumb",
		"SortBy":                  "SortName",
		"SortOrder":               "Ascending",
		"RememberIndexing":        false,
		"PrimaryImageAspectRatio": 1.0,
		"CustomPrefs": map[string]string{
			"views-home":            "view_schedule,view_trending,view_watching",
			"landing-view_schedule": "home",
			"landing-view_trending": "home",
			"landing-view_watching": "home",
		},
		"ScrollDirection": "Horizontal",
		"ShowBackdrop":    true,
		"RememberSorting": false,
	})
}

func (ctrl *ItemController) GetLiveTvChannels(c *gin.Context) {
	c.JSON(http.StatusOK, model.NewQueryResult([]model.BaseItemDto{}))
}

func (ctrl *ItemController) fetchAndServeImage(c *gin.Context, imgUrl string) bool {
	imgUrl = strings.TrimSpace(imgUrl)
	if imgUrl == "" {
		return false
	}
	rewritten := imgUrl
	if ctrl.bangumiClient != nil {
		rewritten = ctrl.bangumiClient.RewriteImageUrl(imgUrl)
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, rewritten, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", bangumi.BangumiUserAgent)
	req.Header.Set("Referer", "https://bgm.tv")

	var resp *http.Response
	if ctrl.bangumiClient != nil {
		resp, err = ctrl.bangumiClient.DoRequest(req)
	} else {
		resp, err = http.DefaultClient.Do(req)
	}

	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		c.Header("Cache-Control", "public, max-age=604800")
		c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, nil)
		return true
	}
	if resp != nil {
		_ = resp.Body.Close()
	}

	// Fallback to original URL if rewritten failed and was different
	if rewritten != imgUrl {
		if reqOrig, errOrig := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, imgUrl, nil); errOrig == nil {
			reqOrig.Header.Set("User-Agent", bangumi.BangumiUserAgent)
			reqOrig.Header.Set("Referer", "https://bgm.tv")
			var respOrig *http.Response
			if ctrl.bangumiClient != nil {
				respOrig, err = ctrl.bangumiClient.DoRequest(reqOrig)
			} else {
				respOrig, err = http.DefaultClient.Do(reqOrig)
			}
			if err == nil && respOrig.StatusCode == http.StatusOK {
				defer respOrig.Body.Close()
				c.Header("Cache-Control", "public, max-age=604800")
				c.DataFromReader(respOrig.StatusCode, respOrig.ContentLength, respOrig.Header.Get("Content-Type"), respOrig.Body, nil)
				return true
			}
			if respOrig != nil {
				_ = respOrig.Body.Close()
			}
		}
	}

	return false
}

func (ctrl *ItemController) GetPrimaryImage(c *gin.Context) {
	id := c.Param("id")

	if strings.HasPrefix(id, "view_") {
		var topSubId int
		if strings.Contains(id, "schedule") {
			if cal, err := ctrl.bangumiClient.GetCalendar(); err == nil && len(cal) > 0 {
				topSubId = cal[0].Id
			}
		} else if strings.Contains(id, "trending") {
			if trending, err := ctrl.bangumiClient.GetTrending(1, 0); err == nil && len(trending) > 0 {
				topSubId = trending[0].Id
			}
		}

		if topSubId > 0 {
			imgUrl := ctrl.bangumiClient.GetCachedImage(topSubId)
			if imgUrl == "" {
				if sub, err := ctrl.bangumiClient.GetSubject(topSubId); err == nil && sub != nil {
					imgUrl = sub.GetPrimaryImage()
				}
			}
			if ctrl.fetchAndServeImage(c, imgUrl) {
				return
			}
		}

		c.Header("Content-Type", "image/png")
		c.Header("Cache-Control", "public, max-age=86400")
		c.Data(http.StatusOK, "image/png", generateFallbackPNG(46, 91, 40, 400, 600))
		return
	}

	subId := mapper.ExtractSubjectId(id)
	if subId == 0 {
		c.Header("Content-Type", "image/png")
		c.Header("Cache-Control", "public, max-age=86400")
		c.Data(http.StatusOK, "image/png", generateFallbackPNG(26, 28, 30, 400, 225))
		return
	}

	imgUrl := ctrl.bangumiClient.GetCachedImage(subId)
	if imgUrl == "" {
		sub, err := ctrl.bangumiClient.GetSubject(subId)
		if err == nil && sub != nil {
			imgUrl = sub.GetPrimaryImage()
		}
	}

	if ctrl.fetchAndServeImage(c, imgUrl) {
		return
	}

	c.Header("Content-Type", "image/png")
	c.Header("Cache-Control", "public, max-age=86400")
	c.Data(http.StatusOK, "image/png", generateFallbackPNG(26, 28, 30, 400, 225))
}

func (ctrl *ItemController) ProxyImage(c *gin.Context) {
	rawUrl := strings.TrimSpace(c.Query("url"))
	if rawUrl == "" {
		c.Status(http.StatusBadRequest)
		return
	}

	u, err := url.Parse(rawUrl)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		c.Status(http.StatusBadRequest)
		return
	}

	if isPrivateOrLoopback(u.Hostname()) {
		c.Status(http.StatusForbidden)
		return
	}

	if ctrl.fetchAndServeImage(c, rawUrl) {
		return
	}

	c.Status(http.StatusBadGateway)
}

func isPrivateOrLoopback(hostname string) bool {
	hostname = strings.TrimSpace(strings.ToLower(hostname))
	if hostname == "localhost" || hostname == "ip6-localhost" || hostname == "ip6-loopback" {
		return true
	}
	ip := net.ParseIP(hostname)
	if ip != nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
	}
	return false
}
