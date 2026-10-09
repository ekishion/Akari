package embyapi

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/singleflight"
	"akari-bridge/internal/bangumi"
	"akari-bridge/internal/config"
	"akari-bridge/internal/danmaku"
	"akari-bridge/internal/engine"
	"akari-bridge/internal/mapper"
	"akari-bridge/internal/model"
	"akari-bridge/internal/resolver"
	"akari-bridge/internal/rules"
	"akari-bridge/internal/storage"
)

type cachedStream struct {
	resolved     *resolver.ResolvedStream
	targetPlugin *engine.Plugin
	expiresAt    time.Time
}

type resolveResult struct {
	resolved     *resolver.ResolvedStream
	targetPlugin *engine.Plugin
	notFound     bool
	errMsg       string
}

type PlaybackHandler struct {
	cfg            *config.Config
	bangumiClient  *bangumi.Client
	ruleMgr        *rules.RuleManager
	engine         *engine.Engine
	streamResolver *resolver.StreamResolver
	danmakuClient  *danmaku.Client
	db             *storage.DB
	sf             singleflight.Group
	cacheMu        sync.RWMutex
	cache          map[string]cachedStream
}

func NewPlaybackHandler(
	cfg *config.Config,
	bangumiClient *bangumi.Client,
	ruleMgr *rules.RuleManager,
	engine *engine.Engine,
	streamResolver *resolver.StreamResolver,
	danmakuClient *danmaku.Client,
	db *storage.DB,
) *PlaybackHandler {
	return &PlaybackHandler{
		cfg:            cfg,
		bangumiClient:  bangumiClient,
		ruleMgr:        ruleMgr,
		engine:         engine,
		streamResolver: streamResolver,
		danmakuClient:  danmakuClient,
		db:             db,
		cache:          make(map[string]cachedStream),
	}
}

func (h *PlaybackHandler) getFromCache(itemId string) (*resolver.ResolvedStream, *engine.Plugin, bool) {
	h.cacheMu.RLock()
	defer h.cacheMu.RUnlock()

	c, ok := h.cache[itemId]
	if !ok || time.Now().After(c.expiresAt) {
		return nil, nil, false
	}
	return c.resolved, c.targetPlugin, true
}

func (h *PlaybackHandler) setToCache(itemId string, res *resolver.ResolvedStream, plugin *engine.Plugin) {
	h.cacheMu.Lock()
	defer h.cacheMu.Unlock()

	h.cache[itemId] = cachedStream{
		resolved:     res,
		targetPlugin: plugin,
		expiresAt:    time.Now().Add(30 * time.Minute),
	}
}

// GetPlaybackInfo handles POST /emby/Items/:id/PlaybackInfo
func (h *PlaybackHandler) GetPlaybackInfo(c *gin.Context) {
	itemId := c.Param("id")

	// 1. Fast path: check in-memory TTL stream cache
	if cachedRes, cachedPlugin, ok := h.getFromCache(itemId); ok && cachedRes != nil && cachedPlugin != nil {
		log.Printf("[Playback] Stream cache HIT for item %s (%s)", itemId, cachedPlugin.Name)
		h.respondPlaybackInfo(c, itemId, cachedRes, cachedPlugin)
		return
	}

	log.Printf("[Playback] Resolving playback for item: %s", itemId)

	title, epIndex, epSort := h.resolveAnimeInfo(itemId)
	if title == "" {
		title = "新番"
	}
	if epIndex <= 0 {
		epIndex = 1
	}

	log.Printf("[Playback] Target: '%s', Episode: %d (Sort: %d)", title, epIndex, epSort)

	// Step 1: Search enabled rules
	enabledPlugins := h.ruleMgr.GetEnabledPlugins()
	if len(enabledPlugins) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "No enabled rules available"})
		return
	}

	var origTitle, cnTitle string
	var extraAliases []string
	var isMovie bool
	subId, _ := mapper.ExtractEpisodeInfo(itemId)
	if subId > 0 {
		if sub, err := h.bangumiClient.GetSubject(subId); err == nil && sub != nil {
			origTitle = sub.Name
			cnTitle = sub.NameCn
			isMovie = sub.IsMovie()
			// Automatically harvest aliases from Bangumi Infobox and Tags
			extraAliases = append(extraAliases, sub.ExtractAliases()...)
		}
		// Also fetch user-configured custom aliases from DB
		if h.db != nil {
			if customAliases, err := h.db.GetSubjectAliases(subId); err == nil && len(customAliases) > 0 {
				extraAliases = append(extraAliases, customAliases...)
			}
		}
	}

	var synonyms map[string]string
	if h.db != nil {
		synonyms, _ = h.db.GetGlobalSynonyms()
	}

	// 2. SingleFlight: coalesce concurrent PlaybackInfo requests for the same item
	val, err, _ := h.sf.Do(itemId, func() (any, error) {
		// Re-check cache inside singleflight
		if cachedRes, cachedPlugin, ok := h.getFromCache(itemId); ok && cachedRes != nil && cachedPlugin != nil {
			return &resolveResult{resolved: cachedRes, targetPlugin: cachedPlugin}, nil
		}

		// Step 1: Perform multi-variant search with dedicated timeout
		searchCtx, cancelSearch := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancelSearch()

		queries := engine.GenerateSearchQueries(title, origTitle, cnTitle, isMovie, synonyms, extraAliases...)
		if len(queries) > 6 {
			queries = queries[:6]
		}
		log.Printf("[Playback] Search queries for '%s' (isMovie=%v): %v", title, isMovie, queries)

		seenCandidate := make(map[string]bool)
		var rawCandidates []engine.SearchItem

		for _, q := range queries {
			results := h.engine.SearchMulti(searchCtx, enabledPlugins, q)
			for _, r := range results {
				key := r.PluginName + ":" + r.Src
				if !seenCandidate[key] {
					seenCandidate[key] = true
					rawCandidates = append(rawCandidates, r)
				}
			}

			// If we found an exact/high-confidence match (score >= 95), stop querying more variants
			if len(rawCandidates) > 0 {
				hasHighConfidence := false
				for _, r := range rawCandidates {
					if engine.ScoreCandidate(title, r.Name, isMovie, synonyms) >= 95 {
						hasHighConfidence = true
						break
					}
				}
				if hasHighConfidence {
					break
				}
			}
		}

		ranked := rankSearchResults(rawCandidates, title, isMovie, synonyms)
		if len(ranked) == 0 {
			log.Printf("[Playback] No valid candidates found matching '%s'", title)
			return &resolveResult{notFound: true, errMsg: "No matching anime source found"}, nil
		}

		log.Printf("[Playback] %d candidate(s) passed matching threshold for '%s', top match: '%s' (%s)",
			len(ranked), title, ranked[0].Name, ranked[0].PluginName)

		// Step 2 & 3: Iterate candidates and resolve with failover
		var workingResolved *resolver.ResolvedStream
		var workingPlugin *engine.Plugin
		var anyCandidateHadEpisode bool

		for _, item := range ranked {
			plugin, ok := h.ruleMgr.GetPluginByName(item.PluginName)
			if !ok {
				continue
			}

			chapterCtx, cancelChapter := context.WithTimeout(context.Background(), 6*time.Second)
			roads, err := h.engine.QueryChapters(chapterCtx, plugin, item.Src)
			cancelChapter()
			if err != nil || len(roads) == 0 {
				log.Printf("[Playback] Rule '%s' chapters query failed for %s: %v", plugin.Name, item.Src, err)
				continue
			}

			// If target is a movie, skip candidates that clearly represent a multi-episode TV series
			if isMovie && len(ranked) > 1 {
				maxRoadEps := 0
				for _, rd := range roads {
					if len(rd.Data) > maxRoadEps {
						maxRoadEps = len(rd.Data)
					}
				}
				if maxRoadEps > 3 {
					log.Printf("[Playback] Candidate '%s' (%s) has %d episodes but target is a movie; skipping to movie candidate", item.Name, plugin.Name, maxRoadEps)
					continue
				}
			}

			playURL := findEpisodeURL(roads, epIndex, epSort)
			if playURL == "" {
				log.Printf("[Playback] Rule '%s' has no episode %d for %s", plugin.Name, epIndex, item.Src)
				continue
			}

			anyCandidateHadEpisode = true

			log.Printf("[Playback] Trying candidate rule '%s' ('%s'), episode page: %s", plugin.Name, item.Name, playURL)
			resolveCtx, cancelResolve := context.WithTimeout(context.Background(), 8*time.Second)
			res, err := h.streamResolver.Resolve(resolveCtx, playURL, plugin.Referer, plugin.UserAgent)
			cancelResolve()

			if err != nil || res == nil || res.RealURL == "" {
				log.Printf("[Playback] Rule '%s' failed to sniff stream (%v), falling back to next candidate...", plugin.Name, err)
				continue
			}

			// Verify whether the stream is actually playable (avoid handing dead/400 URLs to players)
			verifyCtx, cancelVerify := context.WithTimeout(context.Background(), 6*time.Second)
			playableURL, ok := h.streamResolver.VerifyPlayable(verifyCtx, res)
			cancelVerify()
			if !ok {
				log.Printf("[Playback] Rule '%s' stream not playable (%s), falling back to next candidate...", plugin.Name, res.RealURL)
				continue
			}
			res.RealURL = playableURL

			workingResolved = res
			workingPlugin = plugin
			log.Printf("[Playback] Successfully resolved & verified stream from '%s' (%s): %s", plugin.Name, res.Format, res.RealURL)
			break
		}

		if workingResolved == nil {
			if !anyCandidateHadEpisode {
				log.Printf("[Playback] All %d matching sources have not released episode %d yet for '%s'", len(ranked), epIndex, title)
				return &resolveResult{notFound: true, errMsg: fmt.Sprintf("Episode %d is not yet available or released", epIndex)}, nil
			}
			log.Printf("[Playback] All %d candidates failed to resolve video stream for '%s' ep %d", len(ranked), title, epIndex)
			return &resolveResult{notFound: false, errMsg: "Failed to sniff video stream from any source"}, nil
		}

		// Cache resolution
		h.setToCache(itemId, workingResolved, workingPlugin)
		return &resolveResult{resolved: workingResolved, targetPlugin: workingPlugin}, nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result, ok := val.(*resolveResult)
	if !ok || result == nil || result.resolved == nil {
		if result != nil && result.notFound {
			c.JSON(http.StatusNotFound, gin.H{"error": result.errMsg})
			return
		}
		errMsg := "Failed to sniff video stream from any source"
		if result != nil && result.errMsg != "" {
			errMsg = result.errMsg
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": errMsg})
		return
	}

	h.respondPlaybackInfo(c, itemId, result.resolved, result.targetPlugin)
}

func (h *PlaybackHandler) respondPlaybackInfo(c *gin.Context, itemId string, resolved *resolver.ResolvedStream, targetPlugin *engine.Plugin) {
	proxyBase := h.cfg.GetBaseURLFromRequest(c.Request)
	var directStreamURL string

	if resolved.Format == "m3u8" {
		directStreamURL = fmt.Sprintf("%s/stream/m3u8?item_id=%s&url=%s&referer=%s&ua=%s",
			proxyBase, url.QueryEscape(itemId), url.QueryEscape(resolved.RealURL), url.QueryEscape(resolved.Referer), url.QueryEscape(resolved.UserAgent))
	} else {
		directStreamURL = fmt.Sprintf("%s/stream/segment?item_id=%s&url=%s&referer=%s&ua=%s",
			proxyBase, url.QueryEscape(itemId), url.QueryEscape(resolved.RealURL), url.QueryEscape(resolved.Referer), url.QueryEscape(resolved.UserAgent))
	}

	var runTimeTicks int64
	if h.db != nil {
		runTimeTicks = h.db.GetItemTotalTicks(itemId)
	}
	if runTimeTicks <= 0 {
		subId, epIndex := mapper.ExtractEpisodeInfo(itemId)
		epId := mapper.ExtractEpisodeId(itemId)
		if subId > 0 {
			if sub, err := h.bangumiClient.GetSubject(subId); err == nil && sub != nil {
				var targetEp *bangumi.BangumiEpisode
				if epId > 0 {
					if ep, _, err := h.bangumiClient.GetEpisode(epId); err == nil && ep != nil {
						targetEp = ep
					}
				}
				if targetEp == nil {
					if eps, err := h.bangumiClient.GetEpisodes(subId); err == nil {
						for _, ep := range eps {
							if ep.Ep == epIndex || int(ep.Sort) == epIndex {
								targetEp = &ep
								break
							}
						}
					}
				}
				runTimeTicks = mapper.CalculateEpisodeTicks(targetEp, sub)
			}
		}
	}
	if runTimeTicks <= 0 {
		runTimeTicks = 24 * 60 * 1000 * 10000
	}

	subUrl := fmt.Sprintf("%s/emby/Videos/%s/subtitles/0/Stream.ass", proxyBase, itemId)

	container := "hls"
	if resolved.Format == "mp4" {
		container = "mp4"
	}

	resp := model.PlaybackInfoResponse{
		PlaySessionId: "session_" + itemId,
		MediaSources: []model.MediaSourceInfo{
			{
				Id:                   "akari_src_" + targetPlugin.Name,
				Name:                 "Akari · " + targetPlugin.Name,
				Path:                 directStreamURL,
				DirectStreamUrl:      directStreamURL,
				Protocol:             "Http",
				Container:            container,
				Bitrate:              4000000,
				RunTimeTicks:         runTimeTicks,
				SupportsDirectStream: true,
				SupportsDirectPlay:   true,
				SupportsTranscoding:  false,
				MediaStreams: []model.MediaStream{
					{
						Type:     "Video",
						Index:    0,
						Codec:    "h264",
						Width:    1920,
						Height:   1080,
						BitRate:  4000000,
						IsAVC:    true,
					},
					{
						Type:     "Audio",
						Index:    1,
						Codec:    "aac",
						Language: "jpn",
					},
					{
						Type:           "Subtitle",
						Index:          2,
						Codec:          "ass",
						Language:       "chi",
						DisplayTitle:   "弹弹play 弹幕 (ASS)",
						IsExternal:     true,
						DeliveryMethod: "External",
						DeliveryUrl:    subUrl,
						IsDefault:      true,
					},
					{
						Type:           "Subtitle",
						Index:          3,
						Codec:          "vtt",
						Language:       "chi",
						DisplayTitle:   "弹弹play 弹幕 (WebVTT)",
						IsExternal:     true,
						DeliveryMethod: "External",
						DeliveryUrl:    fmt.Sprintf("%s/emby/Videos/%s/subtitles/0/Stream.vtt", proxyBase, itemId),
						IsDefault:      false,
					},
				},
			},
		},
	}

	c.JSON(http.StatusOK, resp)
}

func (h *PlaybackHandler) resolveAnimeInfo(itemId string) (string, int, int) {
	if strings.HasPrefix(itemId, "bgm_ep_") {
		clean := strings.TrimPrefix(itemId, "bgm_ep_")
		parts := strings.Split(clean, "_")
		if len(parts) >= 2 {
			subId, _ := strconv.Atoi(parts[0])
			epIndex, _ := strconv.Atoi(parts[1])
			epSort := epIndex
			if len(parts) >= 3 {
				if epId, err := strconv.Atoi(parts[2]); err == nil && epId > 0 {
					if ep, _, err := h.bangumiClient.GetEpisode(epId); err == nil && ep != nil {
						if ep.Ep > 0 {
							epIndex = ep.Ep
						}
						if int(ep.Sort) > 0 {
							epSort = int(ep.Sort)
						}
					}
				}
			}
			if subId > 0 {
				sub, err := h.bangumiClient.GetSubject(subId)
				if err == nil && sub != nil {
					return sub.GetDisplayName(), epIndex, epSort
				}
			}
			return "", epIndex, epSort
		}

		// Legacy single id: bgm_ep_1741638
		epId, err := strconv.Atoi(parts[0])
		if err == nil && epId > 0 {
			ep, realSubId, err := h.bangumiClient.GetEpisode(epId)
			if err == nil && ep != nil {
				epIndex := ep.Ep
				epSort := int(ep.Sort)
				if epIndex <= 0 {
					epIndex = epSort
				}
				if epIndex <= 0 {
					epIndex = 1
				}
				if realSubId > 0 {
					sub, err := h.bangumiClient.GetSubject(realSubId)
					if err == nil && sub != nil {
						return sub.GetDisplayName(), epIndex, epSort
					}
				}
				return ep.NameCn, epIndex, epSort
			}
		}
	}

	re := regexp.MustCompile(`\d+`)
	matches := re.FindAllString(itemId, -1)
	if len(matches) > 0 {
		if epNum, err := strconv.Atoi(matches[len(matches)-1]); err == nil && epNum > 0 {
			return "", epNum, epNum
		}
	}

	return "", 1, 1
}

// findEpisodeURL picks the play URL for the requested episode, checking both epIndex and epSort.
// It uses strict label parsing to prevent false positional hits or wrong episode playback.
func findEpisodeURL(roads []engine.Road, epIndex int, epSort int) string {
	if len(roads) == 0 {
		return ""
	}

	targets := []int{epIndex}
	if epSort > 0 && epSort != epIndex {
		targets = append(targets, epSort)
	}

	for _, target := range targets {
		// 1. Try exact episode number match from labels first across all roads
		for _, road := range roads {
			for i, name := range road.Identifier {
				if i < len(road.Data) {
					if n, ok := engine.ExtractEpisodeNumber(name); ok && n == target {
						return road.Data[i]
					}
				}
			}
		}

		// 2. Fall back to 1-based positional index if labels don't contradict
		for _, road := range roads {
			idx := target - 1
			if idx >= 0 && idx < len(road.Data) {
				if idx < len(road.Identifier) {
					if labelNum, ok := engine.ExtractEpisodeNumber(road.Identifier[idx]); ok {
						if labelNum != target {
							continue
						}
					}
				}
				return road.Data[idx]
			}
		}
	}

	// 3. Fallback for single-episode / movie / OVA where label is "正片" / "全集" / "播放" etc.
	if epIndex == 1 {
		for _, road := range roads {
			if len(road.Data) > 0 {
				return road.Data[0]
			}
		}
	}

	return ""
}

type scoredCandidate struct {
	item  engine.SearchItem
	score int
}

// rankSearchResults orders search hits by title similarity to the target and filters out mismatches (<40).
func rankSearchResults(items []engine.SearchItem, target string, isMovie bool, synonyms ...map[string]string) []engine.SearchItem {
	if len(items) == 0 {
		return items
	}
	scored := make([]scoredCandidate, 0, len(items))
	for _, it := range items {
		s := engine.ScoreCandidate(target, it.Name, isMovie, synonyms...)
		if s >= 40 {
			scored = append(scored, scoredCandidate{item: it, score: s})
		} else {
			log.Printf("[Playback] Rejected candidate '%s' (%s): score %d < 40 for target '%s'", it.Name, it.PluginName, s, target)
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})
	out := make([]engine.SearchItem, len(scored))
	for i, sc := range scored {
		out[i] = sc.item
	}
	return out
}

// ReportPlaybackPlaying handles POST /emby/Sessions/Playing
type PlaybackProgressReport struct {
	ItemId        string `json:"ItemId"`
	PositionTicks int64  `json:"PositionTicks"`
	TotalTicks    int64  `json:"TotalTicks"`
	Event         string `json:"Event"`
	IsPaused      bool   `json:"IsPaused"`
}

// ReportPlaybackPlaying handles POST /emby/Sessions/Playing
func (h *PlaybackHandler) ReportPlaybackPlaying(c *gin.Context) {
	var req PlaybackProgressReport
	_ = c.ShouldBindJSON(&req)
	userId := c.GetString("userId")
	if req.ItemId != "" && h.db != nil {
		_ = h.db.UpdatePlaybackProgress(userId, req.ItemId, req.PositionTicks, req.TotalTicks)
	}
	c.Status(http.StatusNoContent)
}

// ReportPlaybackProgress handles POST /emby/Sessions/Playing/Progress
func (h *PlaybackHandler) ReportPlaybackProgress(c *gin.Context) {
	var req PlaybackProgressReport
	_ = c.ShouldBindJSON(&req)
	userId := c.GetString("userId")
	if req.ItemId != "" && h.db != nil {
		_ = h.db.UpdatePlaybackProgress(userId, req.ItemId, req.PositionTicks, req.TotalTicks)
	}
	c.Status(http.StatusNoContent)
}

// ReportPlaybackStopped handles POST /emby/Sessions/Playing/Stopped
func (h *PlaybackHandler) ReportPlaybackStopped(c *gin.Context) {
	var req PlaybackProgressReport
	_ = c.ShouldBindJSON(&req)
	userId := c.GetString("userId")
	if req.ItemId != "" && h.db != nil {
		_ = h.db.UpdatePlaybackProgress(userId, req.ItemId, req.PositionTicks, req.TotalTicks)
	}
	c.Status(http.StatusNoContent)
}

// GetAssSubtitleStream handles GET /emby/Videos/:id/subtitles/:index/Stream.ass
func (h *PlaybackHandler) GetAssSubtitleStream(c *gin.Context) {
	itemId := c.Param("id")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	title, epIndex, _ := h.resolveAnimeInfo(itemId)
	if epIndex <= 0 {
		epIndex = 1
	}

	log.Printf("[Danmaku] Fetching danmaku for item: %s ('%s' ep:%d)", itemId, title, epIndex)

	var comments []danmaku.DanmakuComment
	var err error

	// Try Bangumi subject ID
	subId, _ := mapper.ExtractEpisodeInfo(itemId)
	if subId == 0 && strings.HasPrefix(itemId, "bgm_ep_") {
		clean := strings.TrimPrefix(itemId, "bgm_ep_")
		if epId, err := strconv.Atoi(clean); err == nil {
			_, realSubId, err := h.bangumiClient.GetEpisode(epId)
			if err == nil {
				subId = realSubId
			}
		}
	}

	if subId > 0 {
		comments, err = h.danmakuClient.GetCommentsByBgmId(ctx, subId, epIndex)
	}

	if (err != nil || len(comments) == 0) && title != "" {
		comments, _ = h.danmakuClient.SearchAndGetComments(ctx, title, epIndex)
	}

	assContent := danmaku.CommentsToAss(comments, title)

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Cache-Control", "public, max-age=3600")
	c.String(http.StatusOK, assContent)
}

// GetVttSubtitleStream handles GET /emby/Videos/:id/subtitles/:index/Stream.vtt
func (h *PlaybackHandler) GetVttSubtitleStream(c *gin.Context) {
	itemId := c.Param("id")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	title, epIndex, _ := h.resolveAnimeInfo(itemId)
	if epIndex <= 0 {
		epIndex = 1
	}

	var comments []danmaku.DanmakuComment
	subId, _ := mapper.ExtractEpisodeInfo(itemId)
	if subId > 0 {
		comments, _ = h.danmakuClient.GetCommentsByBgmId(ctx, subId, epIndex)
	}
	if len(comments) == 0 && title != "" {
		comments, _ = h.danmakuClient.SearchAndGetComments(ctx, title, epIndex)
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

// SessionCapabilities handles POST /emby/Sessions/Capabilities and /Full
func (h *PlaybackHandler) SessionCapabilities(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// SessionPing handles POST /emby/Sessions/Playing/Ping
func (h *PlaybackHandler) SessionPing(c *gin.Context) {
	c.Status(http.StatusNoContent)
}
