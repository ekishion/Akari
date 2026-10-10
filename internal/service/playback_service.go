package service

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
	"akari-bridge/internal/bangumi"
	"akari-bridge/internal/bilibili"
	"akari-bridge/internal/config"
	"akari-bridge/internal/domain"
	"akari-bridge/internal/engine"
	"akari-bridge/internal/mapper"
	"akari-bridge/internal/model"
	"akari-bridge/internal/resolver"
	"akari-bridge/internal/rules"
)

type cachedStream struct {
	biliStream  *bilibili.ResolvedBiliStream
	ruleStreams []domain.VerifiedRuleStream
	expiresAt   time.Time
}

type PlaybackService struct {
	cfg            *config.Config
	bangumiClient  *bangumi.Client
	ruleMgr        *rules.RuleManager
	engine         *engine.Engine
	streamResolver *resolver.StreamResolver
	playbackRepo   domain.IPlaybackRepository
	synonymRepo    domain.ISynonymRepository
	biliAuth       *bilibili.AuthManager
	biliResolver   *bilibili.Resolver
	dashMuxer      *bilibili.DASHMuxer
	sf             singleflight.Group
	cacheMu        sync.RWMutex
	cache          map[string]cachedStream
}

func NewPlaybackService(
	cfg *config.Config,
	bangumiClient *bangumi.Client,
	ruleMgr *rules.RuleManager,
	engine *engine.Engine,
	streamResolver *resolver.StreamResolver,
	playbackRepo domain.IPlaybackRepository,
	synonymRepo domain.ISynonymRepository,
	biliAuth *bilibili.AuthManager,
	biliResolver *bilibili.Resolver,
	dashMuxer *bilibili.DASHMuxer,
) *PlaybackService {
	return &PlaybackService{
		cfg:            cfg,
		bangumiClient:  bangumiClient,
		ruleMgr:        ruleMgr,
		engine:         engine,
		streamResolver: streamResolver,
		playbackRepo:   playbackRepo,
		synonymRepo:    synonymRepo,
		biliAuth:       biliAuth,
		biliResolver:   biliResolver,
		dashMuxer:      dashMuxer,
		cache:          make(map[string]cachedStream),
	}
}

func (s *PlaybackService) getFromCache(itemId string) (*bilibili.ResolvedBiliStream, []domain.VerifiedRuleStream, bool) {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()

	c, ok := s.cache[itemId]
	if !ok || time.Now().After(c.expiresAt) {
		return nil, nil, false
	}
	return c.biliStream, c.ruleStreams, true
}

func (s *PlaybackService) setToCache(itemId string, biliStream *bilibili.ResolvedBiliStream, ruleStreams []domain.VerifiedRuleStream) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	s.cache[itemId] = cachedStream{
		biliStream:  biliStream,
		ruleStreams: ruleStreams,
		expiresAt:   time.Now().Add(30 * time.Minute),
	}
}

func (s *PlaybackService) ResolvePlayback(ctx context.Context, itemId string) (*domain.PlaybackResolution, error) {
	// 1. Fast path: check in-memory cache
	if cachedBili, cachedRules, ok := s.getFromCache(itemId); ok && (cachedBili != nil || len(cachedRules) > 0) {
		return &domain.PlaybackResolution{
			BiliStream:  cachedBili,
			RuleStreams: cachedRules,
		}, nil
	}

	title, epIndex, epSort, epName := s.resolveAnimeInfo(itemId)
	if title == "" {
		title = "新番"
	}
	if epIndex <= 0 {
		epIndex = 1
	}
	if epSort <= 0 {
		epSort = epIndex
	}

	var origTitle, cnTitle string
	var extraAliases []string
	var isMovie bool
	subId, _ := mapper.ExtractEpisodeInfo(itemId)
	if subId > 0 && s.bangumiClient != nil {
		if sub, err := s.bangumiClient.GetSubject(subId); err == nil && sub != nil {
			origTitle = sub.Name
			cnTitle = sub.NameCn
			isMovie = sub.IsMovie()
			extraAliases = append(extraAliases, sub.ExtractAliases()...)
		}
		if s.synonymRepo != nil {
			if customAliases, err := s.synonymRepo.GetSubjectAliases(subId); err == nil && len(customAliases) > 0 {
				extraAliases = append(extraAliases, customAliases...)
			}
		}
	}

	var synonyms map[string]string
	if s.synonymRepo != nil {
		synonyms, _ = s.synonymRepo.GetGlobalSynonyms()
	}

	var enabledPlugins []*engine.Plugin
	if s.ruleMgr != nil {
		enabledPlugins = s.ruleMgr.GetEnabledPlugins()
	}
	biliEnabled := s.biliAuth != nil && s.biliAuth.GetSettings().Enabled

	if len(enabledPlugins) == 0 && !biliEnabled {
		return nil, fmt.Errorf("no enabled rules or Bilibili source available")
	}

	// 2. SingleFlight: coalesce concurrent requests for same item
	val, err, _ := s.sf.Do(itemId, func() (any, error) {
		if cachedBili, cachedRules, ok := s.getFromCache(itemId); ok && (cachedBili != nil || len(cachedRules) > 0) {
			return &domain.PlaybackResolution{BiliStream: cachedBili, RuleStreams: cachedRules}, nil
		}

		resolveBilibiliFunc := func() *bilibili.ResolvedBiliStream {
			if !biliEnabled || s.biliResolver == nil {
				return nil
			}
			biliSettings := s.biliAuth.GetSettings()
			creds := s.biliAuth.GetCredentials()
			biliCtx, cancelBili := context.WithTimeout(context.Background(), 6*time.Second)
			defer cancelBili()
			bs, err := s.biliResolver.ResolveAnimeEpisode(biliCtx, title, origTitle, cnTitle, epIndex, epSort, isMovie, epName, creds, biliSettings.MaxQuality, biliSettings.StreamMode, extraAliases...)
			if err == nil && bs != nil {
				log.Printf("[Playback] Bilibili successfully resolved for '%s' ep %d: %s (%s)", title, epIndex, bs.Title, bs.QualityLabel)
				return bs
			}
			log.Printf("[Playback] Bilibili resolution skipped/failed for '%s' ep %d: %v", title, epIndex, err)
			return nil
		}

		resolveRulesConcurrent := func() ([]domain.VerifiedRuleStream, bool) {
			if len(enabledPlugins) == 0 {
				return nil, false
			}
			searchCtx, cancelSearch := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancelSearch()

			queries := engine.GenerateSearchQueries(title, origTitle, cnTitle, isMovie, synonyms, extraAliases...)
			if len(queries) > 8 {
				queries = queries[:8]
			}

			seenCandidate := make(map[string]bool)
			var rawCandidates []engine.SearchItem

			for _, q := range queries {
				results := s.engine.SearchMulti(searchCtx, enabledPlugins, q)
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
			if len(ranked) > 6 {
				ranked = ranked[:6]
			}

			var (
				verifiedStreams []domain.VerifiedRuleStream
				verifiedMu      sync.Mutex
				hadEp           bool
				hadEpMu         sync.Mutex
				probeWg         sync.WaitGroup
			)

			for _, sc := range ranked {
				probeWg.Add(1)
				go func(cand scoredCandidate) {
					defer probeWg.Done()

					plugin, ok := s.ruleMgr.GetPluginByName(cand.item.PluginName)
					if !ok {
						return
					}

					candCtx, cancelCand := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancelCand()

					roads, err := s.engine.QueryChapters(candCtx, plugin, cand.item.Src)
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
					res, err := s.streamResolver.Resolve(resolveCtx, playURL, plugin.Referer, plugin.UserAgent)
					cancelResolve()

					if err != nil || res == nil || res.RealURL == "" {
						return
					}

					verifyCtx, cancelVerify := context.WithTimeout(context.Background(), 3*time.Second)
					playableURL, ok := s.streamResolver.VerifyPlayable(verifyCtx, res)
					cancelVerify()
					if !ok {
						return
					}
					res.RealURL = playableURL

					log.Printf("[Playback] Successfully resolved & verified rule stream from '%s' (%s, score=%d): %s", plugin.Name, res.Format, cand.score, res.RealURL)

					verifiedMu.Lock()
					verifiedStreams = append(verifiedStreams, domain.VerifiedRuleStream{
						Plugin:   plugin,
						Resolved: res,
						Score:    cand.score,
					})
					verifiedMu.Unlock()
				}(sc)
			}

			probeWg.Wait()

			sort.SliceStable(verifiedStreams, func(i, j int) bool {
				return verifiedStreams[i].Score > verifiedStreams[j].Score
			})

			return verifiedStreams, hadEp
		}

		var (
			biliStream             *bilibili.ResolvedBiliStream
			ruleStreams            []domain.VerifiedRuleStream
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
				return nil, fmt.Errorf("episode %d is not yet available or released", epIndex)
			}
			return nil, fmt.Errorf("failed to resolve video stream from Bilibili or third-party rules")
		}

		s.setToCache(itemId, biliStream, ruleStreams)
		return &domain.PlaybackResolution{BiliStream: biliStream, RuleStreams: ruleStreams}, nil
	})

	if err != nil {
		return nil, err
	}

	res, ok := val.(*domain.PlaybackResolution)
	if !ok || res == nil {
		return nil, fmt.Errorf("failed to cast resolution result")
	}
	return res, nil
}

func (s *PlaybackService) AssembleMediaSources(itemId string, proxyBase string, res *domain.PlaybackResolution) ([]model.MediaSourceInfo, error) {
	if res == nil {
		return nil, fmt.Errorf("nil playback resolution")
	}

	var runTimeTicks int64
	if s.playbackRepo != nil {
		runTimeTicks = s.playbackRepo.GetItemTotalTicks(itemId)
	}
	if runTimeTicks <= 0 && s.bangumiClient != nil {
		subId, epIndex := mapper.ExtractEpisodeInfo(itemId)
		epId := mapper.ExtractEpisodeId(itemId)
		if subId > 0 {
			if sub, err := s.bangumiClient.GetSubject(subId); err == nil && sub != nil {
				var targetEp *bangumi.BangumiEpisode
				if epId > 0 {
					if ep, _, err := s.bangumiClient.GetEpisode(epId); err == nil && ep != nil {
						targetEp = ep
					}
				}
				if targetEp == nil {
					if eps, err := s.bangumiClient.GetEpisodes(subId); err == nil {
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
	if res.BiliStream != nil {
		var cookieParam string
		if s.biliAuth != nil {
			creds := s.biliAuth.GetCredentials()
			if creds != nil && creds.SessData != "" {
				cookieParam = fmt.Sprintf("&cookie=%s", url.QueryEscape("SESSDATA="+creds.SessData))
			}
		}

		var biliDirectURL string
		if res.BiliStream.IsDASH {
			biliDirectURL = fmt.Sprintf("%s/stream/bilibili/mux?item_id=%s&video_url=%s&audio_url=%s&referer=%s&ua=%s",
				proxyBase, url.QueryEscape(itemId), url.QueryEscape(res.BiliStream.VideoURL), url.QueryEscape(res.BiliStream.AudioURL), url.QueryEscape(res.BiliStream.Referer), url.QueryEscape(res.BiliStream.UserAgent))
		} else {
			biliDirectURL = fmt.Sprintf("%s/stream/segment?item_id=%s&url=%s&referer=%s&ua=%s%s",
				proxyBase, url.QueryEscape(itemId), url.QueryEscape(res.BiliStream.SingleURL), url.QueryEscape(res.BiliStream.Referer), url.QueryEscape(res.BiliStream.UserAgent), cookieParam)
		}

		codec := res.BiliStream.Codec
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
			Name:                 fmt.Sprintf("Akari · 哔哩哔哩 (%s)", res.BiliStream.QualityLabel),
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
	for idx, rStream := range res.RuleStreams {
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
	if s.biliAuth != nil {
		preferBilibili = s.biliAuth.GetSettings().PreferBilibili
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

	return mediaSources, nil
}

func (s *PlaybackService) ReportProgress(userId, itemId string, posTicks, totalTicks int64, isPaused bool) error {
	if s.playbackRepo == nil || itemId == "" {
		return nil
	}
	return s.playbackRepo.UpdatePlaybackProgress(userId, itemId, posTicks, totalTicks)
}

func (s *PlaybackService) ReportStopped(userId, itemId string, posTicks int64) error {
	if s.playbackRepo == nil || itemId == "" {
		return nil
	}
	tot := s.playbackRepo.GetItemTotalTicks(itemId)
	return s.playbackRepo.UpdatePlaybackProgress(userId, itemId, posTicks, tot)
}

func (s *PlaybackService) resolveAnimeInfo(itemId string) (string, int, int, string) {
	if strings.HasPrefix(itemId, "bgm_ep_") {
		clean := strings.TrimPrefix(itemId, "bgm_ep_")
		parts := strings.Split(clean, "_")
		if len(parts) >= 2 {
			subId, _ := strconv.Atoi(parts[0])
			epIndex, _ := strconv.Atoi(parts[1])
			epSort := epIndex
			var epName string
			if len(parts) >= 3 {
				if epId, err := strconv.Atoi(parts[2]); err == nil && epId > 0 && s.bangumiClient != nil {
					if ep, _, err := s.bangumiClient.GetEpisode(epId); err == nil && ep != nil {
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
			if subId > 0 && s.bangumiClient != nil {
				sub, err := s.bangumiClient.GetSubject(subId)
				if err == nil && sub != nil {
					return sub.GetDisplayName(), epIndex, epSort, epName
				}
			}
			return "", epIndex, epSort, epName
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

type scoredCandidate struct {
	item  engine.SearchItem
	score int
}

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

func findEpisodeURL(roads []engine.Road, epIndex int, epSort int) string {
	if len(roads) == 0 {
		return ""
	}

	targets := []int{epIndex}
	if epSort > 0 && epSort != epIndex {
		targets = append(targets, epSort)
	}

	for _, target := range targets {
		for _, road := range roads {
			for i, name := range road.Identifier {
				if i < len(road.Data) {
					if n, ok := engine.ExtractEpisodeNumber(name); ok && n == target {
						return road.Data[i]
					}
				}
			}
		}

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

	if epIndex == 1 {
		for _, road := range roads {
			if len(road.Data) > 0 {
				return road.Data[0]
			}
		}
	}

	return ""
}
