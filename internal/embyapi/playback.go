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
	"akari-bridge/internal/bilibili"
	"akari-bridge/internal/config"
	"akari-bridge/internal/danmaku"
	"akari-bridge/internal/engine"
	"akari-bridge/internal/mapper"
	"akari-bridge/internal/model"
	"akari-bridge/internal/resolver"
	"akari-bridge/internal/rules"
	"akari-bridge/internal/storage"
)

type VerifiedRuleStream struct {
	Plugin   *engine.Plugin            `json:"plugin"`
	Resolved *resolver.ResolvedStream `json:"resolved"`
	Score    int                       `json:"score"`
}

type cachedStream struct {
	biliStream  *bilibili.ResolvedBiliStream
	ruleStreams []VerifiedRuleStream
	expiresAt   time.Time
}

type resolveResult struct {
	biliStream  *bilibili.ResolvedBiliStream
	ruleStreams []VerifiedRuleStream
	notFound    bool
	errMsg      string
}

type PlaybackHandler struct {
	cfg            *config.Config
	bangumiClient  *bangumi.Client
	ruleMgr        *rules.RuleManager
	engine         *engine.Engine
	streamResolver *resolver.StreamResolver
	danmakuClient  *danmaku.Client
	db             *storage.DB
	biliAuth       *bilibili.AuthManager
	biliResolver   *bilibili.Resolver
	dashMuxer      *bilibili.DASHMuxer
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

func (h *PlaybackHandler) SetBilibili(auth *bilibili.AuthManager, res *bilibili.Resolver, muxer *bilibili.DASHMuxer) {
	h.biliAuth = auth
	h.biliResolver = res
	h.dashMuxer = muxer
}

func (h *PlaybackHandler) getFromCache(itemId string) (*bilibili.ResolvedBiliStream, []VerifiedRuleStream, bool) {
	h.cacheMu.RLock()
	defer h.cacheMu.RUnlock()

	c, ok := h.cache[itemId]
	if !ok || time.Now().After(c.expiresAt) {
		return nil, nil, false
	}
	return c.biliStream, c.ruleStreams, true
}

func (h *PlaybackHandler) setToCache(itemId string, biliStream *bilibili.ResolvedBiliStream, ruleStreams []VerifiedRuleStream) {
	h.cacheMu.Lock()
	defer h.cacheMu.Unlock()

	h.cache[itemId] = cachedStream{
		biliStream:  biliStream,
		ruleStreams: ruleStreams,
		expiresAt:   time.Now().Add(30 * time.Minute),
	}
}

// GetPlaybackInfo handles POST /emby/Items/:id/PlaybackInfo
func (h *PlaybackHandler) GetPlaybackInfo(c *gin.Context) {
	itemId := c.Param("id")

	// 1. Fast path: check in-memory TTL stream cache
	if cachedBili, cachedRules, ok := h.getFromCache(itemId); ok && (cachedBili != nil || len(cachedRules) > 0) {
		log.Printf("[Playback] Stream cache HIT for item %s", itemId)
		h.respondPlaybackInfo(c, itemId, cachedBili, cachedRules)
		return
	}

	log.Printf("[Playback] Resolving playback for item: %s", itemId)

	title, epIndex, epSort, epName := h.resolveAnimeInfo(itemId)
	if title == "" {
		title = "新番"
	}
	if epIndex <= 0 {
		epIndex = 1
	}
	if epSort <= 0 {
		epSort = epIndex
	}

	log.Printf("[Playback] Target: '%s', Episode: %d (Sort: %d, Name: '%s')", title, epIndex, epSort, epName)

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

	enabledPlugins := h.ruleMgr.GetEnabledPlugins()
	biliEnabled := h.biliAuth != nil && h.biliAuth.GetSettings().Enabled

	if len(enabledPlugins) == 0 && !biliEnabled {
		c.JSON(http.StatusNotFound, gin.H{"error": "No enabled rules or Bilibili source available"})
		return
	}

	// 2. SingleFlight: coalesce concurrent PlaybackInfo requests for the same item
	val, err, _ := h.sf.Do(itemId, func() (any, error) {
		// Re-check cache inside singleflight
		if cachedBili, cachedRules, ok := h.getFromCache(itemId); ok && (cachedBili != nil || len(cachedRules) > 0) {
			return &resolveResult{biliStream: cachedBili, ruleStreams: cachedRules}, nil
		}

		resolveBilibiliFunc := func() *bilibili.ResolvedBiliStream {
			if !biliEnabled || h.biliResolver == nil {
				return nil
			}
			biliSettings := h.biliAuth.GetSettings()
			creds := h.biliAuth.GetCredentials()
			biliCtx, cancelBili := context.WithTimeout(context.Background(), 6*time.Second)
			defer cancelBili()
			bs, err := h.biliResolver.ResolveAnimeEpisode(biliCtx, title, origTitle, cnTitle, epIndex, epSort, isMovie, epName, creds, biliSettings.MaxQuality, biliSettings.StreamMode, extraAliases...)
			if err == nil && bs != nil {
				log.Printf("[Playback] Bilibili successfully resolved for '%s' ep %d: %s (%s)", title, epIndex, bs.Title, bs.QualityLabel)
				return bs
			}
			log.Printf("[Playback] Bilibili resolution skipped/failed for '%s' ep %d: %v", title, epIndex, err)
			return nil
		}

		resolveRulesConcurrent := func() ([]VerifiedRuleStream, bool) {
			if len(enabledPlugins) == 0 {
				return nil, false
			}
			searchCtx, cancelSearch := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancelSearch()

			queries := engine.GenerateSearchQueries(title, origTitle, cnTitle, isMovie, synonyms, extraAliases...)
			if len(queries) > 8 {
				queries = queries[:8]
			}
			log.Printf("[Playback] Search queries for '%s' (isMovie=%v): %v", title, isMovie, queries)

			seenCandidate := make(map[string]bool)
			var rawCandidates []engine.SearchItem

			// Phase 1: Search all query variations to collect candidate pool across diverse plugins
			for _, q := range queries {
				results := h.engine.SearchMulti(searchCtx, enabledPlugins, q)
				for _, r := range results {
					key := r.PluginName + ":" + r.Src
					if !seenCandidate[key] {
						seenCandidate[key] = true
						rawCandidates = append(rawCandidates, r)
					}
				}
			}

			if len(rawCandidates) == 0 {
				return nil, false
			}

			ranked := rankSearchResultsWithScore(rawCandidates, title, isMovie, synonyms)
			if len(ranked) == 0 {
				return nil, false
			}

			// Keep top candidates for probing (up to top 6)
			if len(ranked) > 6 {
				ranked = ranked[:6]
			}

			var (
				verifiedStreams []VerifiedRuleStream
				verifiedMu      sync.Mutex
				hadEp           bool
				hadEpMu         sync.Mutex
				probeWg         sync.WaitGroup
			)

			// Phase 2: Probe ranked candidates concurrently with timeout
			for _, sc := range ranked {
				probeWg.Add(1)
				go func(cand scoredCandidate) {
					defer probeWg.Done()

					plugin, ok := h.ruleMgr.GetPluginByName(cand.item.PluginName)
					if !ok {
						return
					}

					candCtx, cancelCand := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancelCand()

					roads, err := h.engine.QueryChapters(candCtx, plugin, cand.item.Src)
					if err != nil || len(roads) == 0 {
						return
					}

					if isMovie && len(ranked) > 1 {
						maxRoadEps := 0
						for _, rd := range roads {
							if len(rd.Data) > maxRoadEps {
								maxRoadEps = len(rd.Data)
							}
						}
						if maxRoadEps > 3 {
							return
						}
					}

					playURL := findEpisodeURL(roads, epIndex, epSort)
					if playURL == "" {
						return
					}

					hadEpMu.Lock()
					hadEp = true
					hadEpMu.Unlock()

					resolveCtx, cancelResolve := context.WithTimeout(context.Background(), 5*time.Second)
					res, err := h.streamResolver.Resolve(resolveCtx, playURL, plugin.Referer, plugin.UserAgent)
					cancelResolve()

					if err != nil || res == nil || res.RealURL == "" {
						return
					}

					verifyCtx, cancelVerify := context.WithTimeout(context.Background(), 3*time.Second)
					playableURL, ok := h.streamResolver.VerifyPlayable(verifyCtx, res)
					cancelVerify()
					if !ok {
						return
					}
					res.RealURL = playableURL

					log.Printf("[Playback] Successfully resolved & verified rule stream from '%s' (%s, score=%d): %s", plugin.Name, res.Format, cand.score, res.RealURL)

					verifiedMu.Lock()
					verifiedStreams = append(verifiedStreams, VerifiedRuleStream{
						Plugin:   plugin,
						Resolved: res,
						Score:    cand.score,
					})
					verifiedMu.Unlock()
				}(sc)
			}

			probeWg.Wait()

			// Sort verified streams by Score descending
			sort.SliceStable(verifiedStreams, func(i, j int) bool {
				return verifiedStreams[i].Score > verifiedStreams[j].Score
			})

			return verifiedStreams, hadEp
		}

		var (
			biliStream             *bilibili.ResolvedBiliStream
			ruleStreams            []VerifiedRuleStream
			anyCandidateHadEpisode bool
			mainWg                 sync.WaitGroup
		)

		if biliEnabled {
			mainWg.Add(1)
			go func() {
				defer mainWg.Done()
				biliStream = resolveBilibiliFunc()
			}()
		}

		if len(enabledPlugins) > 0 {
			mainWg.Add(1)
			go func() {
				defer mainWg.Done()
				ruleStreams, anyCandidateHadEpisode = resolveRulesConcurrent()
			}()
		}

		mainWg.Wait()

		if biliStream == nil && len(ruleStreams) == 0 {
			if !anyCandidateHadEpisode && len(enabledPlugins) > 0 {
				log.Printf("[Playback] All matching sources have not released episode %d yet for '%s'", epIndex, title)
				return &resolveResult{notFound: true, errMsg: fmt.Sprintf("Episode %d is not yet available or released", epIndex)}, nil
			}
			log.Printf("[Playback] All sources failed to resolve video stream for '%s' ep %d", title, epIndex)
			return &resolveResult{notFound: false, errMsg: "Failed to resolve video stream from Bilibili or third-party rules"}, nil
		}

		// Cache resolution
		h.setToCache(itemId, biliStream, ruleStreams)
		return &resolveResult{biliStream: biliStream, ruleStreams: ruleStreams}, nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result, ok := val.(*resolveResult)
	if !ok || result == nil || (result.biliStream == nil && len(result.ruleStreams) == 0) {
		if result != nil && result.notFound {
			c.JSON(http.StatusNotFound, gin.H{"error": result.errMsg})
			return
		}
		errMsg := "Failed to resolve video stream from any source"
		if result != nil && result.errMsg != "" {
			errMsg = result.errMsg
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": errMsg})
		return
	}

	h.respondPlaybackInfo(c, itemId, result.biliStream, result.ruleStreams)
}

func (h *PlaybackHandler) respondPlaybackInfo(
	c *gin.Context,
	itemId string,
	biliStream *bilibili.ResolvedBiliStream,
	ruleStreams []VerifiedRuleStream,
) {
	proxyBase := h.cfg.GetBaseURLFromRequest(c.Request)

	var runTimeTicks int64
	if h.db != nil {
		runTimeTicks = h.db.GetItemTotalTicks(itemId)
	}
	if runTimeTicks <= 0 {
		subId, epIndex := mapper.ExtractEpisodeInfo(itemId)
		epId := mapper.ExtractEpisodeId(itemId)
		if subId > 0 && h.bangumiClient != nil {
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
	subVttUrl := fmt.Sprintf("%s/emby/Videos/%s/subtitles/0/Stream.vtt", proxyBase, itemId)

	subtitleStreams := []model.MediaStream{
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
			DeliveryUrl:    subVttUrl,
			IsDefault:      false,
		},
	}

	var biliMediaSource *model.MediaSourceInfo
	if biliStream != nil {
		var cookieParam string
		if h.biliAuth != nil {
			creds := h.biliAuth.GetCredentials()
			if creds != nil && creds.SessData != "" {
				cookieParam = fmt.Sprintf("&cookie=%s", url.QueryEscape("SESSDATA="+creds.SessData))
			}
		}

		var biliDirectURL string
		if biliStream.IsDASH {
			biliDirectURL = fmt.Sprintf("%s/stream/bilibili/mux?item_id=%s&video_url=%s&audio_url=%s&referer=%s&ua=%s",
				proxyBase, url.QueryEscape(itemId), url.QueryEscape(biliStream.VideoURL), url.QueryEscape(biliStream.AudioURL), url.QueryEscape(biliStream.Referer), url.QueryEscape(biliStream.UserAgent))
		} else {
			biliDirectURL = fmt.Sprintf("%s/stream/segment?item_id=%s&url=%s&referer=%s&ua=%s%s",
				proxyBase, url.QueryEscape(itemId), url.QueryEscape(biliStream.SingleURL), url.QueryEscape(biliStream.Referer), url.QueryEscape(biliStream.UserAgent), cookieParam)
		}

		codec := biliStream.Codec
		if codec == "" {
			codec = "h264"
		}
		isAVC := (codec == "h264" || strings.HasPrefix(codec, "avc"))

		streams := []model.MediaStream{
			{
				Type:    "Video",
				Index:   0,
				Codec:   codec,
				Width:   1920,
				Height:  1080,
				BitRate: 6000000,
				IsAVC:   isAVC,
			},
			{
				Type:     "Audio",
				Index:    1,
				Codec:    "aac",
				Language: "jpn",
			},
		}
		streams = append(streams, subtitleStreams...)

		biliMediaSource = &model.MediaSourceInfo{
			Id:                   "akari_src_bilibili",
			Name:                 fmt.Sprintf("Akari · 哔哩哔哩 (%s)", biliStream.QualityLabel),
			Path:                 biliDirectURL,
			DirectStreamUrl:      biliDirectURL,
			Protocol:             "Http",
			Container:            "mp4",
			Bitrate:              6000000,
			RunTimeTicks:         runTimeTicks,
			SupportsDirectStream: true,
			SupportsDirectPlay:   true,
			SupportsTranscoding:  false,
			MediaStreams:         streams,
		}
	}

	var ruleMediaSources []model.MediaSourceInfo
	for idx, rStream := range ruleStreams {
		if rStream.Plugin == nil || rStream.Resolved == nil {
			continue
		}
		var directStreamURL string
		container := "hls"
		if rStream.Resolved.Format == "m3u8" {
			directStreamURL = fmt.Sprintf("%s/stream/m3u8?item_id=%s&url=%s&referer=%s&ua=%s",
				proxyBase, url.QueryEscape(itemId), url.QueryEscape(rStream.Resolved.RealURL), url.QueryEscape(rStream.Resolved.Referer), url.QueryEscape(rStream.Resolved.UserAgent))
		} else {
			container = "mp4"
			directStreamURL = fmt.Sprintf("%s/stream/segment?item_id=%s&url=%s&referer=%s&ua=%s",
				proxyBase, url.QueryEscape(itemId), url.QueryEscape(rStream.Resolved.RealURL), url.QueryEscape(rStream.Resolved.Referer), url.QueryEscape(rStream.Resolved.UserAgent))
		}

		formatLabel := strings.ToUpper(rStream.Resolved.Format)
		if formatLabel == "" {
			formatLabel = "HLS"
		}

		streams := []model.MediaStream{
			{
				Type:    "Video",
				Index:   0,
				Codec:   "h264",
				Width:   1920,
				Height:  1080,
				BitRate: 4000000,
				IsAVC:   true,
			},
			{
				Type:     "Audio",
				Index:    1,
				Codec:    "aac",
				Language: "jpn",
			},
		}
		streams = append(streams, subtitleStreams...)

		sourceId := fmt.Sprintf("akari_src_%s_%d", rStream.Plugin.Name, idx+1)
		sourceName := fmt.Sprintf("Akari · %s (%s)", rStream.Plugin.Name, formatLabel)

		ruleMediaSources = append(ruleMediaSources, model.MediaSourceInfo{
			Id:                   sourceId,
			Name:                 sourceName,
			Path:                 directStreamURL,
			DirectStreamUrl:      directStreamURL,
			Protocol:             "Http",
			Container:            container,
			Bitrate:              4000000,
			RunTimeTicks:         runTimeTicks,
			SupportsDirectStream: true,
			SupportsDirectPlay:   true,
			SupportsTranscoding:  false,
			MediaStreams:         streams,
		})
	}

	var mediaSources []model.MediaSourceInfo
	preferBilibili := true
	if h.biliAuth != nil {
		preferBilibili = h.biliAuth.GetSettings().PreferBilibili
	}

	if preferBilibili {
		if biliMediaSource != nil {
			mediaSources = append(mediaSources, *biliMediaSource)
		}
		mediaSources = append(mediaSources, ruleMediaSources...)
	} else {
		mediaSources = append(mediaSources, ruleMediaSources...)
		if biliMediaSource != nil {
			mediaSources = append(mediaSources, *biliMediaSource)
		}
	}

	resp := model.PlaybackInfoResponse{
		PlaySessionId: "session_" + itemId,
		MediaSources:  mediaSources,
	}

	c.JSON(http.StatusOK, resp)
}

func (h *PlaybackHandler) resolveAnimeInfo(itemId string) (string, int, int, string) {
	if strings.HasPrefix(itemId, "bgm_ep_") {
		clean := strings.TrimPrefix(itemId, "bgm_ep_")
		parts := strings.Split(clean, "_")
		if len(parts) >= 2 {
			subId, _ := strconv.Atoi(parts[0])
			epIndex, _ := strconv.Atoi(parts[1])
			epSort := epIndex
			var epName string
			if len(parts) >= 3 {
				if epId, err := strconv.Atoi(parts[2]); err == nil && epId > 0 {
					if ep, _, err := h.bangumiClient.GetEpisode(epId); err == nil && ep != nil {
						if ep.Ep > 0 {
							epIndex = ep.Ep
						}
						if int(ep.Sort) > 0 {
							epSort = int(ep.Sort)
						}
						epName = ep.NameCn
						if epName == "" {
							epName = ep.Name
						}
					}
				}
			}
			if subId > 0 {
				sub, err := h.bangumiClient.GetSubject(subId)
				if err == nil && sub != nil {
					return sub.GetDisplayName(), epIndex, epSort, epName
				}
			}
			return "", epIndex, epSort, epName
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
				epName := ep.NameCn
				if epName == "" {
					epName = ep.Name
				}
				if realSubId > 0 {
					sub, err := h.bangumiClient.GetSubject(realSubId)
					if err == nil && sub != nil {
						return sub.GetDisplayName(), epIndex, epSort, epName
					}
				}
				return ep.NameCn, epIndex, epSort, epName
			}
		}
	}

	re := regexp.MustCompile(`\d+`)
	matches := re.FindAllString(itemId, -1)
	if len(matches) > 0 {
		if epNum, err := strconv.Atoi(matches[len(matches)-1]); err == nil && epNum > 0 {
			return "", epNum, epNum, ""
		}
	}

	return "", 1, 1, ""
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

// rankSearchResultsWithScore orders search hits by title similarity to the target and filters out mismatches (<40).
func rankSearchResultsWithScore(items []engine.SearchItem, target string, isMovie bool, synonyms ...map[string]string) []scoredCandidate {
	if len(items) == 0 {
		return nil
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
	return scored
}

// rankSearchResults orders search hits by title similarity to the target and filters out mismatches (<40).
func rankSearchResults(items []engine.SearchItem, target string, isMovie bool, synonyms ...map[string]string) []engine.SearchItem {
	scored := rankSearchResultsWithScore(items, target, isMovie, synonyms...)
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

	title, epIndex, _, _ := h.resolveAnimeInfo(itemId)
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

	title, epIndex, _, _ := h.resolveAnimeInfo(itemId)
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

// HandleBilibiliMux handles GET /stream/bilibili/mux and streams transmuxed MP4 via ffmpeg
func (h *PlaybackHandler) HandleBilibiliMux(c *gin.Context) {
	if h.dashMuxer == nil || !h.dashMuxer.IsAvailable() {
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
	if h.biliAuth != nil {
		creds := h.biliAuth.GetCredentials()
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

	if err := h.dashMuxer.MuxStream(c.Request.Context(), c.Writer, videoURL, audioURL, referer, ua, cookie, startSeconds); err != nil {
		log.Printf("[Playback] DASH mux stream finished: %v", err)
	}
}

