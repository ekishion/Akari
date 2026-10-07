package embyapi

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"akari-bridge/internal/auth"
	"akari-bridge/internal/bangumi"
	"akari-bridge/internal/config"
	"akari-bridge/internal/mapper"
	"akari-bridge/internal/model"
	"akari-bridge/internal/storage"
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

type ItemsHandler struct {
	cfg           *config.Config
	bangumiClient *bangumi.Client
	authSvc       *auth.AuthService
	db            *storage.DB
}

func NewItemsHandler(cfg *config.Config, bangumiClient *bangumi.Client, authSvc *auth.AuthService, db *storage.DB) *ItemsHandler {
	return &ItemsHandler{
		cfg:           cfg,
		bangumiClient: bangumiClient,
		authSvc:       authSvc,
		db:            db,
	}
}

// GetUserItems handles GET /emby/Users/:id/Items
func (h *ItemsHandler) GetUserItems(c *gin.Context) {
	userId := c.Param("id")
	if userId == "" {
		userId = c.GetString("userId")
	}
	parentId := c.Query("ParentId")
	searchTerm := c.Query("SearchTerm")
	filters := c.Query("Filters")
	limitStr := c.DefaultQuery("Limit", "50")
	startIndexStr := c.DefaultQuery("StartIndex", "0")
	limit, _ := strconv.Atoi(limitStr)
	startIndex, _ := strconv.Atoi(startIndexStr)
	if limit <= 0 {
		limit = 50
	}

	// 1. If filtering favorites
	isFav := c.Query("IsFavorite")
	includeItemTypes := c.Query("IncludeItemTypes")
	if strings.Contains(filters, "IsFavorite") || isFav == "true" || isFav == "1" {
		var favItems []model.BaseItemDto
		seen := make(map[string]bool)

		// 1. Load locally favorited items from SQLite (primary and authoritative)
		if h.db != nil {
			localFavIds := h.db.GetUserFavoriteItemIDs(userId, limit+50)
			for _, fid := range localFavIds {
				if strings.HasPrefix(fid, "bgm_sub_") || strings.HasPrefix(fid, "bgm_ep_") || strings.HasPrefix(fid, "bgm_season_") {
					if matchItemType("Series", includeItemTypes) {
						sid := mapper.ExtractSubjectId(fid)
						seriesId := fmt.Sprintf("bgm_sub_%d", sid)
						if sid > 0 && !seen[seriesId] {
							seen[seriesId] = true
							if sub, err := h.bangumiClient.GetSubject(sid); err == nil && sub != nil {
								item := mapper.SubjectToSeries(sub, h.cfg.ServerId)
								item.UserData = h.db.GetUserItemData(userId, seriesId)
								if item.UserData == nil || !item.UserData.IsFavorite {
									item.UserData = &model.UserItemDataDto{Key: seriesId, IsFavorite: true}
								}
								favItems = append(favItems, item)
							}
						}
					}
				}
				if strings.HasPrefix(fid, "bgm_ep_") {
					if matchItemType("Episode", includeItemTypes) {
						subId, epIndex := mapper.ExtractEpisodeInfo(fid)
						epId := mapper.ExtractEpisodeId(fid)
						if sub, err := h.bangumiClient.GetSubject(subId); err == nil && sub != nil {
							var targetEp *bangumi.BangumiEpisode
							if epId > 0 {
								if ep, _, err := h.bangumiClient.GetEpisode(epId); err == nil && ep != nil {
									targetEp = ep
								}
							}
							if targetEp == nil {
								if episodes, err := h.bangumiClient.GetEpisodes(subId); err == nil {
									for _, ep := range episodes {
										if ep.Ep == epIndex || int(ep.Sort) == epIndex {
											targetEp = &ep
											break
										}
									}
								}
							}
							if targetEp != nil {
								item := mapper.EpisodeToEmbyEpisode(targetEp, sub, h.cfg.ServerId)
								if h.db != nil {
									item.UserData = h.db.GetUserItemData(userId, fid)
									if totalTicks := h.db.GetItemTotalTicks(item.Id); totalTicks > 0 {
										item.RunTimeTicks = totalTicks
									}
								}
								if !seen[item.Id] {
									seen[item.Id] = true
									favItems = append(favItems, item)
								}
							}
						}
					}
				}
			}
		}

		// 2. Supplemental: if user has bound Bangumi personal access token, append Bangumi Wish (Type 1)
		if h.authSvc != nil && len(favItems) < limit && matchItemType("Series", includeItemTypes) {
			bgmToken, bgmUid := h.authSvc.GetUserBangumi(userId)
			if bgmToken != "" {
				userTarget := bgmUid
				if userTarget == "" {
					userTarget = "@me"
				}
				if subjects, err := h.bangumiClient.GetUserCollections(userTarget, bgmToken, 1 /* Wish */, limit, startIndex); err == nil && len(subjects) > 0 {
					for _, sub := range subjects {
						item := mapper.SubjectToSeries(&sub, h.cfg.ServerId)
						if !seen[item.Id] {
							seen[item.Id] = true
							if h.db != nil {
								item.UserData = h.db.GetUserItemData(userId, item.Id)
							} else {
								item.UserData = &model.UserItemDataDto{IsFavorite: true}
							}
							favItems = append(favItems, item)
						}
					}
				}
			}
		}

		totalCount := len(favItems)
		var pagedItems []model.BaseItemDto
		if startIndex < totalCount {
			end := startIndex + limit
			if end > totalCount {
				end = totalCount
			}
			pagedItems = favItems[startIndex:end]
		} else {
			pagedItems = []model.BaseItemDto{}
		}

		c.JSON(http.StatusOK, model.QueryResult[model.BaseItemDto]{
			Items:            pagedItems,
			TotalRecordCount: totalCount,
		})
		return
	}

	// 2. Specific Provider ID probe
	if c.Query("AnyProviderIdEquals") != "" {
		c.JSON(http.StatusOK, model.NewQueryResult([]model.BaseItemDto{}))
		return
	}

	items := make([]model.BaseItemDto, 0)

	// 3. If searching
	if searchTerm != "" {
		subjects, err := h.bangumiClient.Search(searchTerm, limit)
		if err == nil {
			for _, sub := range subjects {
				item := mapper.SubjectToSeries(&sub, h.cfg.ServerId)
				if h.db != nil {
					item.UserData = h.db.GetUserItemData(userId, item.Id)
				}
				items = append(items, item)
			}
		}
		c.JSON(http.StatusOK, model.QueryResult[model.BaseItemDto]{
			Items:            items,
			TotalRecordCount: len(items),
		})
		return
	}

	// 4. By View / ParentId
	if strings.HasPrefix(parentId, "bgm_sub_") {
		subId := mapper.ExtractSubjectId(parentId)
		sub, err := h.bangumiClient.GetSubject(subId)
		if err == nil && sub != nil {
			season := mapper.SubjectToSeason(sub, h.cfg.ServerId)
			if h.db != nil {
				season.UserData = h.db.GetUserItemData(userId, season.Id)
			}
			items = append(items, season)
		}
	} else if strings.HasPrefix(parentId, "bgm_season_") {
		subId := mapper.ExtractSubjectId(parentId)
		sub, err := h.bangumiClient.GetSubject(subId)
		if err == nil && sub != nil {
			episodes, err := h.bangumiClient.GetEpisodes(subId)
			if err == nil {
				for _, ep := range episodes {
					if !mapper.IsEpisodeAired(&ep, sub) {
						continue
					}
					epItem := mapper.EpisodeToEmbyEpisode(&ep, sub, h.cfg.ServerId)
					if h.db != nil {
						epItem.UserData = h.db.GetUserItemData(userId, epItem.Id)
						if totalTicks := h.db.GetItemTotalTicks(epItem.Id); totalTicks > 0 {
							epItem.RunTimeTicks = totalTicks
						}
					}
					items = append(items, epItem)
				}
			}
		}
	} else {
		switch parentId {
		case mapper.ViewIdSchedule:
			subjects, err := h.bangumiClient.GetCalendar()
			if err == nil {
				for _, sub := range subjects {
					item := mapper.SubjectToSeries(&sub, h.cfg.ServerId)
					if h.db != nil {
						item.UserData = h.db.GetUserItemData(userId, item.Id)
					}
					items = append(items, item)
				}
			}
		case mapper.ViewIdTrending, "":
			fetchLimit := limit + startIndex
			if fetchLimit < 30 {
				fetchLimit = 30
			}
			subjects, err := h.bangumiClient.GetTrending(fetchLimit, 0)
			if err == nil {
				for _, sub := range subjects {
					item := mapper.SubjectToSeries(&sub, h.cfg.ServerId)
					if h.db != nil {
						item.UserData = h.db.GetUserItemData(userId, item.Id)
					}
					items = append(items, item)
				}
			}
		case mapper.ViewIdWatching:
			var userSubjects []bangumi.BangumiSubject
			seenSubject := make(map[int]bool)

			// 1. Primary: Load active watching anime (Favorited + Played / In-Progress) from SQLite
			if h.db != nil {
				watchingSubIds := h.db.GetUserWatchingSubjectIDs(userId, limit+20)
				for _, sid := range watchingSubIds {
					if !seenSubject[sid] {
						if sub, err := h.bangumiClient.GetSubject(sid); err == nil && sub != nil {
							seenSubject[sid] = true
							userSubjects = append(userSubjects, *sub)
						}
					}
				}
			}

			// 2. Supplemental: If user has bound Bangumi account, merge Bangumi Watching (CollectionType 3 / Do)
			if h.authSvc != nil {
				bgmToken, bgmUid := h.authSvc.GetUserBangumi(userId)
				if bgmUid != "" || bgmToken != "" {
					userTarget := bgmUid
					if userTarget == "" {
						userTarget = "@me"
					}
					if subs, err := h.bangumiClient.GetUserCollections(userTarget, bgmToken, 3 /* Watching */, limit, startIndex); err == nil && len(subs) > 0 {
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
				item := mapper.SubjectToSeries(&sub, h.cfg.ServerId)
				if h.db != nil {
					item.UserData = h.db.GetUserItemData(userId, item.Id)
				}
				items = append(items, item)
			}
		}
	}

	totalCount := len(items)
	var pagedItems []model.BaseItemDto
	if startIndex < totalCount {
		end := startIndex + limit
		if end > totalCount {
			end = totalCount
		}
		pagedItems = items[startIndex:end]
	} else {
		pagedItems = []model.BaseItemDto{}
	}

	c.JSON(http.StatusOK, model.NewQueryResult(pagedItems))
}

// GetItemCounts handles GET /emby/Items/Counts
func (h *ItemsHandler) GetItemCounts(c *gin.Context) {
	historyCount := 0
	if h.db != nil {
		if stats, err := h.db.GetStats(); err == nil && stats != nil {
			historyCount = stats.HistoryCount
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

// GetResumeItems handles GET /emby/Users/:id/Items/Resume
func (h *ItemsHandler) GetResumeItems(c *gin.Context) {
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
	if h.db != nil {
		itemIds := h.db.GetUserResumeItems(userId, limit)
		for _, id := range itemIds {
			if strings.HasPrefix(id, "bgm_ep_") {
				subId, epIndex := mapper.ExtractEpisodeInfo(id)
				epId := mapper.ExtractEpisodeId(id)
				if subId > 0 {
					sub, err := h.bangumiClient.GetSubject(subId)
					if err == nil && sub != nil {
						var targetEp *bangumi.BangumiEpisode
						if epId > 0 {
							if ep, _, err := h.bangumiClient.GetEpisode(epId); err == nil && ep != nil {
								targetEp = ep
							}
						}
						if targetEp == nil {
							episodes, err := h.bangumiClient.GetEpisodes(subId)
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
							item := mapper.EpisodeToEmbyEpisode(targetEp, sub, h.cfg.ServerId)
							if h.db != nil {
								item.UserData = h.db.GetUserItemData(userId, id)
								if totalTicks := h.db.GetItemTotalTicks(item.Id); totalTicks > 0 {
									item.RunTimeTicks = totalTicks
								}
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

// GetStudios handles GET /emby/Studios
func (h *ItemsHandler) GetStudios(c *gin.Context) {
	c.JSON(http.StatusOK, model.NewQueryResult([]model.BaseItemDto{}))
}

// GetGenres handles GET /emby/Genres
func (h *ItemsHandler) GetGenres(c *gin.Context) {
	c.JSON(http.StatusOK, model.NewQueryResult([]model.BaseItemDto{}))
}

// GetNextUp handles GET /emby/Shows/NextUp
func (h *ItemsHandler) GetNextUp(c *gin.Context) {
	userId := c.Query("UserId")
	if userId == "" {
		userId = c.GetString("userId")
	}
	seriesId := c.Query("SeriesId")

	items := make([]model.BaseItemDto, 0)
	if seriesId != "" && strings.HasPrefix(seriesId, "bgm_sub_") {
		subId := mapper.ExtractSubjectId(seriesId)
		if subId > 0 {
			sub, err := h.bangumiClient.GetSubject(subId)
			if err == nil && sub != nil {
				episodes, err := h.bangumiClient.GetEpisodes(subId)
				if err == nil && len(episodes) > 0 {
					for _, ep := range episodes {
						if !mapper.IsEpisodeAired(&ep, sub) {
							continue
						}
						epIndex := ep.Ep
						if epIndex <= 0 {
							epIndex = int(ep.Sort)
						}
						if epIndex <= 0 {
							epIndex = 1
						}
						epItemId := fmt.Sprintf("bgm_ep_%d_%d_%d", sub.Id, epIndex, ep.Id)
						userData := h.db.GetUserItemData(userId, epItemId)
						if userData == nil || !userData.Played {
							item := mapper.EpisodeToEmbyEpisode(&ep, sub, h.cfg.ServerId)
							if h.db != nil {
								item.UserData = userData
								if totalTicks := h.db.GetItemTotalTicks(item.Id); totalTicks > 0 {
									item.RunTimeTicks = totalTicks
								}
							}
							items = append(items, item)
							break
						}
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, model.NewQueryResult(items))
}

// GetItem handles GET /emby/Items/:id and /emby/Users/:id/Items/:itemId
func (h *ItemsHandler) GetItem(c *gin.Context) {
	id := c.Param("itemId")
	if id == "" {
		id = c.Param("id")
	}
	userId := c.GetString("userId")
	if userId == "" {
		userId = c.Param("id")
	}

	// Check if requesting view folder
	views := mapper.CreateVirtualViews(h.cfg.ServerId)
	for _, v := range views {
		if v.Id == id {
			c.JSON(http.StatusOK, v)
			return
		}
	}

	// Series
	if strings.HasPrefix(id, "bgm_sub_") {
		subId := mapper.ExtractSubjectId(id)
		sub, err := h.bangumiClient.GetSubject(subId)
		if err == nil && sub != nil {
			item := mapper.SubjectToSeries(sub, h.cfg.ServerId)
			if h.db != nil {
				item.UserData = h.db.GetUserItemData(userId, id)
			}
			c.JSON(http.StatusOK, item)
			return
		}
	}

	// Season
	if strings.HasPrefix(id, "bgm_season_") {
		subId := mapper.ExtractSubjectId(id)
		sub, err := h.bangumiClient.GetSubject(subId)
		if err == nil && sub != nil {
			season := mapper.SubjectToSeason(sub, h.cfg.ServerId)
			if h.db != nil {
				season.UserData = h.db.GetUserItemData(userId, id)
			}
			c.JSON(http.StatusOK, season)
			return
		}
	}

	// Episode
	if strings.HasPrefix(id, "bgm_ep_") {
		subId, epIndex := mapper.ExtractEpisodeInfo(id)
		epId := mapper.ExtractEpisodeId(id)
		if subId > 0 {
			sub, err := h.bangumiClient.GetSubject(subId)
			if err == nil && sub != nil {
				var targetEp *bangumi.BangumiEpisode
				if epId > 0 {
					if ep, _, err := h.bangumiClient.GetEpisode(epId); err == nil && ep != nil {
						targetEp = ep
					}
				}
				if targetEp == nil {
					if episodes, err := h.bangumiClient.GetEpisodes(subId); err == nil {
						for _, ep := range episodes {
							if ep.Ep == epIndex || int(ep.Sort) == epIndex {
								targetEp = &ep
								break
							}
						}
					}
				}
				if targetEp == nil {
					targetEp = &bangumi.BangumiEpisode{
						Id:     epId,
						Ep:     epIndex,
						NameCn: fmt.Sprintf("第 %d 集", epIndex),
					}
				}
				epItem := mapper.EpisodeToEmbyEpisode(targetEp, sub, h.cfg.ServerId)
				epItem.Id = id
				if h.db != nil {
					epItem.UserData = h.db.GetUserItemData(userId, id)
					if totalTicks := h.db.GetItemTotalTicks(id); totalTicks > 0 {
						epItem.RunTimeTicks = totalTicks
					}
				}
				c.JSON(http.StatusOK, epItem)
				return
			}
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
}

// GetSeasons handles GET /emby/Shows/:id/Seasons
func (h *ItemsHandler) GetSeasons(c *gin.Context) {
	seriesId := c.Param("id")
	subId := mapper.ExtractSubjectId(seriesId)
	userId := c.GetString("userId")
	if userId == "" {
		userId = c.Query("UserId")
	}

	sub, err := h.bangumiClient.GetSubject(subId)
	if err != nil || sub == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Series not found"})
		return
	}

	season := mapper.SubjectToSeason(sub, h.cfg.ServerId)
	if h.db != nil {
		season.UserData = h.db.GetUserItemData(userId, season.Id)
	}
	items := []model.BaseItemDto{season}
	c.JSON(http.StatusOK, model.NewQueryResult(items))
}

// GetEpisodes handles GET /emby/Shows/:id/Episodes
func (h *ItemsHandler) GetEpisodes(c *gin.Context) {
	seriesId := c.Param("id")
	subId := mapper.ExtractSubjectId(seriesId)
	userId := c.GetString("userId")
	if userId == "" {
		userId = c.Query("UserId")
	}

	sub, err := h.bangumiClient.GetSubject(subId)
	if err != nil || sub == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Series not found"})
		return
	}

	episodes, err := h.bangumiClient.GetEpisodes(subId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load episodes"})
		return
	}

	items := make([]model.BaseItemDto, 0)
	for _, ep := range episodes {
		if !mapper.IsEpisodeAired(&ep, sub) {
			continue
		}
		item := mapper.EpisodeToEmbyEpisode(&ep, sub, h.cfg.ServerId)
		if h.db != nil {
			item.UserData = h.db.GetUserItemData(userId, item.Id)
			if totalTicks := h.db.GetItemTotalTicks(item.Id); totalTicks > 0 {
				item.RunTimeTicks = totalTicks
			}
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, model.NewQueryResult(items))
}

// GetThemeMedia handles GET /emby/Items/:id/ThemeMedia and ThemeSongs
func (h *ItemsHandler) GetThemeMedia(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"ThemeVideosResult":     model.NewQueryResult([]model.BaseItemDto{}),
		"ThemeSongsResult":      model.NewQueryResult([]model.BaseItemDto{}),
		"SoundtrackSongsResult": model.NewQueryResult([]model.BaseItemDto{}),
	})
}

// GetSimilarItems handles GET /emby/Items/:id/Similar
func (h *ItemsHandler) GetSimilarItems(c *gin.Context) {
	c.JSON(http.StatusOK, model.NewQueryResult([]model.BaseItemDto{}))
}

// GetLatestItems handles GET /emby/Users/:id/Items/Latest and /emby/Items/Latest
// Standard Emby returns []BaseItemDto array
func (h *ItemsHandler) GetLatestItems(c *gin.Context) {
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

	switch parentId {
	case mapper.ViewIdSchedule:
		subjects, err := h.bangumiClient.GetCalendar()
		if err == nil {
			for _, sub := range subjects {
				item := mapper.SubjectToSeries(&sub, h.cfg.ServerId)
				if h.db != nil {
					item.UserData = h.db.GetUserItemData(userId, item.Id)
				}
				items = append(items, item)
			}
		}
	case mapper.ViewIdWatching:
		var userSubjects []bangumi.BangumiSubject
		seenSubject := make(map[int]bool)

		if h.db != nil {
			watchingSubIds := h.db.GetUserWatchingSubjectIDs(userId, limit+10)
			for _, sid := range watchingSubIds {
				if !seenSubject[sid] {
					if sub, err := h.bangumiClient.GetSubject(sid); err == nil && sub != nil {
						seenSubject[sid] = true
						userSubjects = append(userSubjects, *sub)
					}
				}
			}
		}

		if h.authSvc != nil {
			bgmToken, bgmUid := h.authSvc.GetUserBangumi(userId)
			if bgmUid != "" || bgmToken != "" {
				userTarget := bgmUid
				if userTarget == "" {
					userTarget = "@me"
				}
				if subs, err := h.bangumiClient.GetUserCollections(userTarget, bgmToken, 3, limit, 0); err == nil && len(subs) > 0 {
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
			item := mapper.SubjectToSeries(&sub, h.cfg.ServerId)
			if h.db != nil {
				item.UserData = h.db.GetUserItemData(userId, item.Id)
			}
			items = append(items, item)
		}
	case mapper.ViewIdTrending, "":
		fallthrough
	default:
		fetchLimit := limit
		if fetchLimit < 20 {
			fetchLimit = 20
		}
		subjects, err := h.bangumiClient.GetTrending(fetchLimit, 0)
		if err == nil {
			for _, sub := range subjects {
				item := mapper.SubjectToSeries(&sub, h.cfg.ServerId)
				if h.db != nil {
					item.UserData = h.db.GetUserItemData(userId, item.Id)
				}
				items = append(items, item)
			}
		}
	}

	if len(items) > limit {
		items = items[:limit]
	}

	c.JSON(http.StatusOK, items)
}

// GetPrimaryImage handles GET /emby/Items/:id/Images/Primary
func (h *ItemsHandler) GetPrimaryImage(c *gin.Context) {
	id := c.Param("id")

	// 1. If library view folder, proxy top anime cover or return PNG
	if strings.HasPrefix(id, "view_") {
		var topSubId int
		if strings.Contains(id, "schedule") {
			if cal, err := h.bangumiClient.GetCalendar(); err == nil && len(cal) > 0 {
				topSubId = cal[0].Id
			}
		} else if strings.Contains(id, "trending") {
			if trending, err := h.bangumiClient.GetTrending(1, 0); err == nil && len(trending) > 0 {
				topSubId = trending[0].Id
			}
		} else if strings.Contains(id, "watching") {
			if h.db != nil {
				watching := h.db.GetUserWatchingSubjectIDs("admin", 1)
				if len(watching) > 0 {
					topSubId = watching[0]
				}
			}
			if topSubId == 0 {
				if trending, err := h.bangumiClient.GetTrending(2, 0); err == nil && len(trending) > 1 {
					topSubId = trending[1].Id
				}
			}
		}

		if topSubId > 0 {
			imgUrl := h.bangumiClient.GetCachedImage(topSubId)
			if imgUrl == "" {
				if sub, err := h.bangumiClient.GetSubject(topSubId); err == nil && sub != nil {
					imgUrl = sub.GetPrimaryImage()
				}
			}
			if imgUrl != "" {
				if req, err := http.NewRequest(http.MethodGet, imgUrl, nil); err == nil {
					req.Header.Set("User-Agent", bangumi.BangumiUserAgent)
					req.Header.Set("Referer", "https://bgm.tv")
					if resp, err := http.DefaultClient.Do(req); err == nil && resp.StatusCode == http.StatusOK {
						defer resp.Body.Close()
						c.Header("Cache-Control", "public, max-age=86400")
						c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, nil)
						return
					}
				}
			}
		}

		// Fallback PNG
		c.Header("Content-Type", "image/png")
		c.Header("Cache-Control", "public, max-age=86400")
		c.Data(http.StatusOK, "image/png", generateFallbackPNG(46, 91, 40, 400, 600))
		return
	}

	// 2. Anime Series / Episode Primary Image
	subId := mapper.ExtractSubjectId(id)
	if subId == 0 && strings.HasPrefix(id, "bgm_ep_") {
		epIdStr := strings.TrimPrefix(id, "bgm_ep_")
		if epId, err := strconv.Atoi(epIdStr); err == nil {
			_, realSubId, err := h.bangumiClient.GetEpisode(epId)
			if err == nil {
				subId = realSubId
			}
		}
	}

	if subId == 0 {
		c.Header("Content-Type", "image/png")
		c.Header("Cache-Control", "public, max-age=86400")
		c.Data(http.StatusOK, "image/png", generateFallbackPNG(26, 28, 30, 400, 225))
		return
	}

	// Check fast memory image map
	imgUrl := h.bangumiClient.GetCachedImage(subId)
	if imgUrl == "" {
		sub, err := h.bangumiClient.GetSubject(subId)
		if err == nil && sub != nil {
			imgUrl = sub.GetPrimaryImage()
		}
	}

	if imgUrl == "" {
		c.Header("Content-Type", "image/png")
		c.Header("Cache-Control", "public, max-age=86400")
		c.Data(http.StatusOK, "image/png", generateFallbackPNG(26, 28, 30, 400, 225))
		return
	}

	// Proxy image bytes
	req, err := http.NewRequest(http.MethodGet, imgUrl, nil)
	if err != nil {
		c.Status(http.StatusBadGateway)
		return
	}
	req.Header.Set("User-Agent", bangumi.BangumiUserAgent)
	req.Header.Set("Referer", "https://bgm.tv")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.Status(http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	c.Header("Cache-Control", "public, max-age=604800")
	c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, nil)
}

// ProxyImage handles GET /emby/items/images/proxy
func (h *ItemsHandler) ProxyImage(c *gin.Context) {
	rawUrl := c.Query("url")
	if rawUrl == "" {
		c.Status(http.StatusBadRequest)
		return
	}

	req, err := http.NewRequest(http.MethodGet, rawUrl, nil)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", "https://bgm.tv")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.Status(http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		if strings.HasPrefix(strings.ToLower(k), "content-") {
			c.Header(k, v[0])
		}
	}
	c.Header("Cache-Control", "public, max-age=604800")
	_, _ = io.Copy(c.Writer, resp.Body)
}

func matchItemType(itemType, includeItemTypes string) bool {
	if includeItemTypes == "" {
		return true
	}
	types := strings.Split(includeItemTypes, ",")
	for _, t := range types {
		if strings.EqualFold(strings.TrimSpace(t), itemType) {
			return true
		}
	}
	return false
}

// GetPersons handles GET /emby/Persons and /emby/Persons/:name
func (h *ItemsHandler) GetPersons(c *gin.Context) {
	c.JSON(http.StatusOK, model.NewQueryResult([]model.BaseItemDto{}))
}

// GetCollections handles GET /emby/Collections
func (h *ItemsHandler) GetCollections(c *gin.Context) {
	c.JSON(http.StatusOK, model.NewQueryResult([]model.BaseItemDto{}))
}

// GetPlaylists handles GET /emby/Playlists
func (h *ItemsHandler) GetPlaylists(c *gin.Context) {
	c.JSON(http.StatusOK, model.NewQueryResult([]model.BaseItemDto{}))
}

// GetDisplayPreferences handles GET & POST /emby/DisplayPreferences/:id
func (h *ItemsHandler) GetDisplayPreferences(c *gin.Context) {
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

// GetLiveTvChannels handles GET /emby/LiveTv/Channels
func (h *ItemsHandler) GetLiveTvChannels(c *gin.Context) {
	c.JSON(http.StatusOK, model.NewQueryResult([]model.BaseItemDto{}))
}
