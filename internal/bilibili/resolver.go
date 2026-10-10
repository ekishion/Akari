package bilibili

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	htmlTagRegex        = regexp.MustCompile(`<[^>]*>`)
	seasonNumRegex       = regexp.MustCompile(`第([0-9一二三四五六七八九十]+)[季期部]`)
	seasonRegexChinese   = regexp.MustCompile(`(?i)第\s*([一二三四五六七八九十0-9]+)\s*[季期部]`)
	seasonRegexEnglish   = regexp.MustCompile(`(?i)(?:season|s)\s*([0-9]+)`)
	seasonRegexOrdinal   = regexp.MustCompile(`(?i)([0-9]+)(?:st|nd|rd|th)\s*season`)
	seasonRegexTrailing  = regexp.MustCompile(`(?:\D|^)([0-9]+)$`)
	romanRegex2          = regexp.MustCompile(`(?i)(?:[\s_.\-:])+(?:ii|ⅱ)(?:[\s_.\-:]|$)`)
	romanRegex3          = regexp.MustCompile(`(?i)(?:[\s_.\-:])+(?:iii|ⅲ)(?:[\s_.\-:]|$)`)
	romanRegex4          = regexp.MustCompile(`(?i)(?:[\s_.\-:])+(?:iv|ⅳ)(?:[\s_.\-:]|$)`)
	romanRegex5          = regexp.MustCompile(`(?i)(?:[\s_.\-:])+(?:v|ⅴ)(?:[\s_.\-:]|$)`)
	seasonChineseMap     = map[string]int{
		"一": 1, "二": 2, "三": 3, "四": 4, "五": 5, "六": 6, "七": 7, "八": 8, "九": 9, "十": 10,
		"1": 1, "2": 2, "3": 3, "4": 4, "5": 5, "6": 6, "7": 7, "8": 8, "9": 9, "10": 10,
	}
)

type Resolver struct {
	client *Client
}

func NewResolver(client *Client) *Resolver {
	if client == nil {
		client = NewClient(nil)
	}
	return &Resolver{client: client}
}

// ResolveAnimeEpisode searches Bilibili for matching PGC anime/movie and resolves stream (supports direct MP4 and DASH)
func (r *Resolver) ResolveAnimeEpisode(
	ctx context.Context,
	title, origTitle, cnTitle string,
	epIndex, epSort int,
	isMovie bool,
	epName string,
	creds *BilibiliCredentials,
	maxQn int,
	streamMode string,
	aliases ...string,
) (*ResolvedBiliStream, error) {
	if epIndex <= 0 {
		epIndex = 1
	}
	if epSort <= 0 {
		epSort = epIndex
	}
	if maxQn <= 0 {
		maxQn = 120 // Highest quality default (4K / 1080P+ / 1080P)
	}
	if streamMode == "" {
		streamMode = "direct"
	}

	searchKeywords := generateBilibiliSearchKeywords(title, origTitle, cnTitle, isMovie, aliases...)
	log.Printf("[BilibiliResolver] Searching anime for '%s' ep %d (sort %d, isMovie=%v) with keywords: %v", title, epIndex, epSort, isMovie, searchKeywords)

	var searchTypes []string
	if isMovie {
		searchTypes = []string{"media_ft", "media_bangumi"}
	} else {
		searchTypes = []string{"media_bangumi", "media_ft"}
	}

	var bestSeasonID int
	var bestSeasonTitle string
	var highestScore float64

	for _, kw := range searchKeywords {
		for _, stype := range searchTypes {
			searchResp, err := r.client.SearchPGC(ctx, kw, stype, creds)
			if err != nil || searchResp == nil || len(searchResp.Data.Result) == 0 {
				continue
			}

			for _, item := range searchResp.Data.Result {
				cleanItemTitle := stripHTMLTags(item.Title)
				cleanOrgTitle := stripHTMLTags(item.OrgTitle)
				score := scoreTitleMatch(title, cnTitle, origTitle, isMovie, cleanItemTitle, cleanOrgTitle, item.MediaType, item.SeasonType)
				if score > highestScore {
					highestScore = score
					bestSeasonID = item.SeasonID
					bestSeasonTitle = cleanItemTitle
				}
			}

			if highestScore >= 0.88 {
				break
			}
		}
		if highestScore >= 0.88 {
			break
		}
	}

	if bestSeasonID == 0 || highestScore < 0.4 {
		return nil, fmt.Errorf("no matching bilibili anime found for '%s'", title)
	}

	log.Printf("[BilibiliResolver] Matched anime '%s' (season_id=%d, score=%.2f)", bestSeasonTitle, bestSeasonID, highestScore)

	// Fetch Season Episodes
	seasonResp, err := r.client.GetSeason(ctx, bestSeasonID, 0, creds)
	if err != nil || seasonResp == nil || len(seasonResp.Result.Episodes) == 0 {
		return nil, fmt.Errorf("failed to fetch season episodes for season_id %d: %v", bestSeasonID, err)
	}

	targetEpID, targetBvid, targetCid, epTitle := findTargetEpisode(seasonResp.Result.Episodes, epIndex, epSort, epName, isMovie)
	if targetEpID == 0 && targetCid == 0 {
		return nil, fmt.Errorf("episode %d (sort %d) not found in bilibili season '%s'", epIndex, epSort, bestSeasonTitle)
	}

	log.Printf("[BilibiliResolver] Found ep %d (sort %d, ep_id=%d, bvid=%s, cid=%d, title='%s')", epIndex, epSort, targetEpID, targetBvid, targetCid, epTitle)

	fnval := 0 // Default to MP4 single stream (免 ffmpeg 零依赖直链模式)
	if streamMode == "dash" {
		fnval = 4048 // DASH multi-stream
	}

	// Fetch PlayURL
	var playRes *PlayURLResponse
	if targetEpID > 0 {
		playRes, err = r.client.GetPGCPlayURL(ctx, targetEpID, maxQn, fnval, creds)
		if (err != nil || playRes == nil || (len(playRes.Result.Durl) == 0 && playRes.Result.Dash == nil && len(playRes.Data.Durl) == 0 && playRes.Data.Dash == nil)) && fnval == 0 {
			playRes, err = r.client.GetPGCPlayURL(ctx, targetEpID, maxQn, 4048, creds)
		}
	} else if targetBvid != "" && targetCid > 0 {
		playRes, err = r.client.GetUGCPlayURL(ctx, targetBvid, targetCid, maxQn, fnval, creds)
		if (err != nil || playRes == nil || (len(playRes.Result.Durl) == 0 && playRes.Result.Dash == nil && len(playRes.Data.Durl) == 0 && playRes.Data.Dash == nil)) && fnval == 0 {
			playRes, err = r.client.GetUGCPlayURL(ctx, targetBvid, targetCid, maxQn, 4048, creds)
		}
	}

	if err != nil || playRes == nil {
		return nil, fmt.Errorf("failed to get bilibili playurl: %v", err)
	}

	// Extract result
	resolved := extractPlayableStream(playRes, maxQn)
	if resolved == nil {
		return nil, fmt.Errorf("no playable stream found in bilibili playurl response")
	}

	// Verify duration: full TV anime episodes are typically > 3 minutes (180s = 180000ms), except short movies/trailers
	if !isMovie && resolved.DurationMs > 0 && resolved.DurationMs < 120000 {
		return nil, fmt.Errorf("matched stream duration (%d ms) is too short (< 2 min), likely a preview/trailer", resolved.DurationMs)
	}

	resolved.Title = fmt.Sprintf("%s - %s", bestSeasonTitle, epTitle)
	resolved.Referer = fmt.Sprintf("https://www.bilibili.com/bangumi/play/ep%d", targetEpID)
	resolved.UserAgent = r.client.userAgent

	log.Printf("[BilibiliResolver] Resolved stream for '%s' [qn=%d, %s, codec=%s, isDASH=%v]", resolved.Title, resolved.Quality, resolved.QualityLabel, resolved.Codec, resolved.IsDASH)
	return resolved, nil
}

func generateBilibiliSearchKeywords(title, origTitle, cnTitle string, isMovie bool, aliases ...string) []string {
	seen := make(map[string]bool)
	var list []string

	add := func(s string) {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] && len([]rune(s)) >= 2 {
			seen[s] = true
			list = append(list, s)
		}
	}

	add(cnTitle)
	add(title)
	add(origTitle)
	for _, a := range aliases {
		add(a)
	}

	// Clean titles without season suffixes (e.g. "第二季" -> "")
	if cnTitle != "" {
		add(seasonNumRegex.ReplaceAllString(cnTitle, ""))
	}
	if title != "" {
		add(seasonNumRegex.ReplaceAllString(title, ""))
	}

	if isMovie {
		if cnTitle != "" {
			add(cnTitle + " 剧场版")
			add(cnTitle + " 电影")
		}
		if title != "" {
			add(title + " 剧场版")
			add(title + " 电影")
		}
	}

	if len(list) > 8 {
		list = list[:8]
	}
	return list
}

func stripHTMLTags(s string) string {
	return strings.TrimSpace(htmlTagRegex.ReplaceAllString(s, ""))
}

func extractSeasonNumber(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if m := seasonRegexChinese.FindStringSubmatch(s); len(m) > 1 {
		if val, ok := seasonChineseMap[m[1]]; ok {
			return val
		}
		if n, err := strconv.Atoi(m[1]); err == nil {
			return n
		}
	}
	if m := seasonRegexEnglish.FindStringSubmatch(s); len(m) > 1 {
		if n, err := strconv.Atoi(m[1]); err == nil {
			return n
		}
	}
	if m := seasonRegexOrdinal.FindStringSubmatch(s); len(m) > 1 {
		if n, err := strconv.Atoi(m[1]); err == nil {
			return n
		}
	}
	if romanRegex5.MatchString(s) {
		return 5
	}
	if romanRegex4.MatchString(s) {
		return 4
	}
	if romanRegex3.MatchString(s) {
		return 3
	}
	if romanRegex2.MatchString(s) {
		return 2
	}
	if m := seasonRegexTrailing.FindStringSubmatch(s); len(m) > 1 {
		if n, err := strconv.Atoi(m[1]); err == nil && n >= 2 && n <= 20 {
			return n
		}
	}
	return 0
}

func isSeasonMismatch(targetSeason, candidateSeason int) bool {
	if targetSeason > 0 && candidateSeason > 0 {
		return targetSeason != candidateSeason
	}
	if targetSeason > 1 && candidateSeason == 0 {
		return true
	}
	if targetSeason <= 1 && candidateSeason > 1 {
		return true
	}
	return false
}

func cleanTitleForComparison(s string) string {
	s = seasonRegexChinese.ReplaceAllString(s, "")
	s = seasonRegexEnglish.ReplaceAllString(s, "")
	s = seasonRegexOrdinal.ReplaceAllString(s, "")
	s = romanRegex2.ReplaceAllString(s, " ")
	s = romanRegex3.ReplaceAllString(s, " ")
	s = romanRegex4.ReplaceAllString(s, " ")
	s = romanRegex5.ReplaceAllString(s, " ")
	s = seasonRegexTrailing.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "剧场版", "")
	s = strings.ReplaceAll(s, "大电影", "")
	s = strings.ReplaceAll(s, "电影", "")
	return strings.TrimSpace(s)
}

func runeOverlapScore(a, b string) float64 {
	runesA := []rune(a)
	runesB := []rune(b)
	if len(runesA) == 0 || len(runesB) == 0 {
		return 0
	}
	setB := make(map[rune]bool)
	for _, r := range runesB {
		setB[r] = true
	}
	common := 0
	for _, r := range runesA {
		if setB[r] {
			common++
		}
	}
	if common == 0 {
		return 0
	}
	ratioA := float64(common) / float64(len(runesA))
	ratioB := float64(common) / float64(len(runesB))
	return (ratioA + ratioB) / 2.0
}

func scoreTitleMatch(target1, target2, target3 string, isMovie bool, candidate1, candidate2 string, candidateMediaType, candidateSeasonType int) float64 {
	targets := []string{target1, target2, target3}
	candidates := []string{candidate1, candidate2}

	targetSeason := 0
	for _, t := range targets {
		if s := extractSeasonNumber(t); s > 0 {
			targetSeason = s
			break
		}
	}

	candidateSeason := 0
	for _, c := range candidates {
		if s := extractSeasonNumber(c); s > 0 {
			candidateSeason = s
			break
		}
	}

	if isSeasonMismatch(targetSeason, candidateSeason) {
		return 0.0
	}

	isCandidateMovie := (candidateMediaType == 2 || candidateSeasonType == 2 ||
		strings.Contains(candidate1, "剧场版") || strings.Contains(candidate1, "电影") || strings.Contains(candidate1, "大电影") || strings.Contains(candidate1, "Movie"))

	var maxScore float64
	for _, t := range targets {
		if t == "" {
			continue
		}
		cleanT := strings.ToLower(stripPunctuation(cleanTitleForComparison(t)))
		for _, c := range candidates {
			if c == "" {
				continue
			}
			cleanC := strings.ToLower(stripPunctuation(cleanTitleForComparison(c)))
			if cleanT == "" || cleanC == "" {
				continue
			}
			if cleanT == cleanC {
				maxScore = 1.0
				break
			}
			if strings.Contains(cleanC, cleanT) || strings.Contains(cleanT, cleanC) {
				score := float64(len(cleanT)) / float64(len(cleanC))
				if score > 1.0 {
					score = 1.0 / score
				}
				if score > maxScore {
					maxScore = score
				}
			} else {
				overlap := runeOverlapScore(cleanT, cleanC)
				if overlap >= 0.6 && overlap > maxScore {
					maxScore = overlap
				}
			}
		}
		if maxScore >= 1.0 {
			break
		}
	}

	if maxScore < 0.3 {
		return maxScore
	}

	// Adjust score based on Movie vs TV matching
	if isMovie {
		if isCandidateMovie {
			maxScore += 0.20
			if maxScore > 1.0 {
				maxScore = 1.0
			}
		} else {
			// Severe penalty if user wanted a movie but candidate is a TV anime
			maxScore -= 0.45
			if maxScore < 0 {
				maxScore = 0
			}
		}
	} else {
		if isCandidateMovie {
			// Penalty if user wanted a TV anime but candidate is a movie
			maxScore -= 0.35
			if maxScore < 0 {
				maxScore = 0
			}
		}
	}

	return maxScore
}

func stripPunctuation(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || (r >= 0x4e00 && r <= 0x9fa5) || (r >= 0x3040 && r <= 0x30ff) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isTrailerBadgeOrTitle(title, longTitle, badge string) bool {
	b := strings.TrimSpace(badge)
	if b == "预告" || b == "PV" || b == "特报" || b == "花絮" || b == "OP" || b == "ED" || b == "CM" {
		return true
	}
	comb := strings.ToLower(title + " " + longTitle)
	if strings.Contains(comb, "预告") || strings.Contains(comb, "pv") || strings.Contains(comb, "特报") || strings.Contains(comb, "制作决定") || strings.Contains(comb, "告知") {
		return true
	}
	return false
}

func findTargetEpisode(
	episodes []struct {
		ID        int    `json:"id"`
		Aid       int64  `json:"aid"`
		Bvid      string `json:"bvid"`
		Cid       int64  `json:"cid"`
		Title     string `json:"title"`
		LongTitle string `json:"long_title"`
		Badge     string `json:"badge"`
		ShareURL  string `json:"share_url"`
	},
	epIndex int,
	epSort int,
	epName string,
	isMovie bool,
) (int, string, int64, string) {
	// 1. Movie handling: return the feature film episode ("正片", "原版", "日配", "国配", "中文", "全片" or first non-trailer)
	if isMovie || len(episodes) <= 2 {
		for _, ep := range episodes {
			if isTrailerBadgeOrTitle(ep.Title, ep.LongTitle, ep.Badge) {
				continue
			}
			t := strings.TrimSpace(ep.Title)
			if t == "正片" || t == "原版" || t == "全片" || t == "1" || t == "01" {
				name := ep.Title
				if ep.LongTitle != "" {
					name += " " + ep.LongTitle
				}
				return ep.ID, ep.Bvid, ep.Cid, name
			}
		}
		// Return first non-trailer
		for _, ep := range episodes {
			if !isTrailerBadgeOrTitle(ep.Title, ep.LongTitle, ep.Badge) {
				name := ep.Title
				if ep.LongTitle != "" {
					name += " " + ep.LongTitle
				}
				return ep.ID, ep.Bvid, ep.Cid, name
			}
		}
	}

	// 2. Exact Title Match on epSort if epSort is different from epIndex (e.g. 众生之门篇 ep 1 has Sort 29)
	if epSort > 0 && epSort != epIndex {
		sortStr := strconv.Itoa(epSort)
		sortStrPadded := fmt.Sprintf("%02d", epSort)
		for _, ep := range episodes {
			if isTrailerBadgeOrTitle(ep.Title, ep.LongTitle, ep.Badge) {
				continue
			}
			cleanTitle := strings.TrimSpace(ep.Title)
			if cleanTitle == sortStr || cleanTitle == sortStrPadded {
				name := cleanTitle
				if ep.LongTitle != "" {
					name += " " + ep.LongTitle
				}
				return ep.ID, ep.Bvid, ep.Cid, name
			}
		}
	}

	// 3. Match by epName / LongTitle if epName is provided (e.g. "异界中的灵力")
	if epName != "" {
		cleanTargetName := stripPunctuation(strings.ToLower(epName))
		if len([]rune(cleanTargetName)) >= 2 {
			for _, ep := range episodes {
				if isTrailerBadgeOrTitle(ep.Title, ep.LongTitle, ep.Badge) {
					continue
				}
				cleanEpLong := stripPunctuation(strings.ToLower(ep.LongTitle))
				cleanEpTitle := stripPunctuation(strings.ToLower(ep.Title))
				if cleanEpLong == cleanTargetName || strings.Contains(cleanEpLong, cleanTargetName) || strings.Contains(cleanTargetName, cleanEpLong) ||
					cleanEpTitle == cleanTargetName || strings.Contains(cleanEpTitle, cleanTargetName) {
					name := ep.Title
					if ep.LongTitle != "" {
						name += " " + ep.LongTitle
					}
					return ep.ID, ep.Bvid, ep.Cid, name
				}
			}
		}
	}

	// 4. Exact match on epIndex as number (e.g. Title == "1" or "01") while ignoring trailers
	epStr := strconv.Itoa(epIndex)
	epStrPadded := fmt.Sprintf("%02d", epIndex)
	for _, ep := range episodes {
		if isTrailerBadgeOrTitle(ep.Title, ep.LongTitle, ep.Badge) {
			continue
		}
		cleanTitle := strings.TrimSpace(ep.Title)
		if cleanTitle == epStr || cleanTitle == epStrPadded {
			name := cleanTitle
			if ep.LongTitle != "" {
				name += " " + ep.LongTitle
			}
			return ep.ID, ep.Bvid, ep.Cid, name
		}
	}

	// 5. Filter out non-trailers and use positional 1-based index
	var validEpisodes []struct {
		ID        int    `json:"id"`
		Aid       int64  `json:"aid"`
		Bvid      string `json:"bvid"`
		Cid       int64  `json:"cid"`
		Title     string `json:"title"`
		LongTitle string `json:"long_title"`
		Badge     string `json:"badge"`
		ShareURL  string `json:"share_url"`
	}
	for _, ep := range episodes {
		if !isTrailerBadgeOrTitle(ep.Title, ep.LongTitle, ep.Badge) {
			validEpisodes = append(validEpisodes, ep)
		}
	}

	if len(validEpisodes) == 0 {
		return 0, "", 0, ""
	}

	if epIndex >= 1 && epIndex <= len(validEpisodes) {
		ep := validEpisodes[epIndex-1]
		name := ep.Title
		if ep.LongTitle != "" {
			name += " " + ep.LongTitle
		}
		return ep.ID, ep.Bvid, ep.Cid, name
	}

	// Fallback for ep 1
	if epIndex == 1 && len(validEpisodes) > 0 {
		ep := validEpisodes[0]
		name := ep.Title
		if ep.LongTitle != "" {
			name += " " + ep.LongTitle
		}
		return ep.ID, ep.Bvid, ep.Cid, name
	}

	return 0, "", 0, ""
}

func codecScore(codec string) int {
	c := strings.ToLower(codec)
	if strings.HasPrefix(c, "avc") || strings.Contains(c, "h264") {
		return 10 // H.264 / AVC has 100% universal hardware playback support across all players
	}
	if strings.HasPrefix(c, "hev") || strings.HasPrefix(c, "hvc") || strings.Contains(c, "h265") || strings.Contains(c, "hevc") {
		return 8 // HEVC / H.265
	}
	if strings.HasPrefix(c, "av01") || strings.Contains(c, "av1") {
		return 1 // AV1 (low priority due to mobile hardware decoder limitations)
	}
	return 0
}

// IsOfficialCDN returns whether a URL is hosted on official Bilibili UPOS CDN
func IsOfficialCDN(rawURL string) bool {
	if rawURL == "" {
		return false
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	// Disallow MCDN / P2P / third-party edge nodes
	if strings.Contains(host, "mcdn") || strings.Contains(host, "mountaintoys") || strings.Contains(host, "pcdn") || strings.Contains(host, "szbdyd") || strings.Contains(host, "tc.ourdvsss.com") {
		return false
	}
	// Disallow non-standard ports (MCDN often uses :4480, :4483, :4484)
	if u.Port() != "" && u.Port() != "80" && u.Port() != "443" {
		return false
	}
	// Accept official UPOS CDNs
	return strings.HasSuffix(host, ".bilivideo.com") || strings.HasSuffix(host, ".biliapi.net") || strings.HasSuffix(host, ".akamaized.net")
}

// SanitizeUposURL filters out slow or unreachable PCDN/MCDN nodes and ensures official CDN mirrors
func SanitizeUposURL(rawURL string, backupURLs []string) string {
	if rawURL == "" {
		return ""
	}
	if IsOfficialCDN(rawURL) {
		return rawURL
	}
	for _, b := range backupURLs {
		if IsOfficialCDN(b) {
			log.Printf("[BilibiliResolver] Swapped PCDN/MCDN URL to official CDN backup: %s", b)
			return b
		}
	}
	u, err := url.Parse(rawURL)
	if err == nil && u.Path != "" {
		u.Scheme = "https"
		u.Host = "upos-sz-mirrorcos.bilivideo.com"
		log.Printf("[BilibiliResolver] Rewrote PCDN/MCDN host to official UPOS mirror: %s", u.String())
		return u.String()
	}
	return rawURL
}

func extractPlayableStream(res *PlayURLResponse, maxQn int) *ResolvedBiliStream {
	if res == nil {
		return nil
	}

	dash := res.Result.Dash
	if dash == nil {
		dash = res.Data.Dash
	}

	durl := res.Result.Durl
	if len(durl) == 0 {
		durl = res.Data.Durl
	}

	quality := res.Result.Quality
	if quality == 0 {
		quality = res.Data.Quality
	}

	duration := res.Result.Timelength
	if duration == 0 {
		duration = res.Data.Timelength
	}

	// 1. Try DASH stream
	if dash != nil && len(dash.Video) > 0 && len(dash.Audio) > 0 {
		// Filter and sort video streams by quality <= maxQn
		var eligibleVideos []DashStream
		for _, v := range dash.Video {
			if maxQn <= 0 || v.ID <= maxQn {
				eligibleVideos = append(eligibleVideos, v)
			}
		}
		if len(eligibleVideos) == 0 {
			eligibleVideos = dash.Video
		}

		sort.Slice(eligibleVideos, func(i, j int) bool {
			scoreI := codecScore(eligibleVideos[i].Codecs)
			scoreJ := codecScore(eligibleVideos[j].Codecs)
			if eligibleVideos[i].ID != eligibleVideos[j].ID {
				// Prefer AVC/HEVC (score > 1) if both have good quality (>= 80 1080P)
				if (scoreI == 1 && scoreJ > 1) || (scoreJ == 1 && scoreI > 1) {
					if eligibleVideos[i].ID >= 80 && eligibleVideos[j].ID >= 80 {
						return scoreI > scoreJ
					}
				}
				return eligibleVideos[i].ID > eligibleVideos[j].ID
			}
			if scoreI != scoreJ {
				return scoreI > scoreJ // Prioritize H.264/AVC for universal compatibility
			}
			return eligibleVideos[i].Bandwidth > eligibleVideos[j].Bandwidth
		})

		bestVideo := eligibleVideos[0]

		// Select highest bandwidth audio stream
		sort.Slice(dash.Audio, func(i, j int) bool {
			return dash.Audio[i].Bandwidth > dash.Audio[j].Bandwidth
		})
		bestAudio := dash.Audio[0]

		codecName := "h264"
		if strings.HasPrefix(strings.ToLower(bestVideo.Codecs), "hev") || strings.HasPrefix(strings.ToLower(bestVideo.Codecs), "hvc") {
			codecName = "hevc"
		} else if strings.HasPrefix(strings.ToLower(bestVideo.Codecs), "av01") {
			codecName = "av1"
		}

		videoURL := SanitizeUposURL(bestVideo.BaseURL, bestVideo.BackupURL)
		audioURL := SanitizeUposURL(bestAudio.BaseURL, bestAudio.BackupURL)

		return &ResolvedBiliStream{
			Quality:      bestVideo.ID,
			QualityLabel: getQualityLabel(bestVideo.ID),
			Codec:        codecName,
			VideoURL:     videoURL,
			AudioURL:     audioURL,
			IsDASH:       true,
			DurationMs:   duration,
		}
	}

	// 2. Fallback to single MP4/FLV durl stream
	if len(durl) > 0 && durl[0].URL != "" {
		singleURL := SanitizeUposURL(durl[0].URL, durl[0].BackupURL)
		return &ResolvedBiliStream{
			Quality:      quality,
			QualityLabel: getQualityLabel(quality),
			Codec:        "h264",
			SingleURL:    singleURL,
			IsDASH:       false,
			DurationMs:   duration,
		}
	}

	return nil
}

func getQualityLabel(qn int) string {
	switch qn {
	case 127:
		return "8K 超高清"
	case 126:
		return "杜比视界"
	case 125:
		return "HDR 真彩"
	case 120:
		return "4K 超清"
	case 116:
		return "1080P 60帧"
	case 112:
		return "1080P 高码率"
	case 80:
		return "1080P 高清"
	case 64:
		return "720P 高清"
	case 32:
		return "480P 清晰"
	case 16:
		return "360P 流畅"
	default:
		return fmt.Sprintf("%dP", qn)
	}
}
