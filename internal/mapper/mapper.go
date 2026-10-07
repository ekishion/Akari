package mapper

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"akari-bridge/internal/bangumi"
	"akari-bridge/internal/model"
)

const (
	ViewIdWatching = "view_watching"
	ViewIdSchedule = "view_schedule"
	ViewIdTrending = "view_trending"
)

// FormatEmbyDate parses various date strings and returns standard RFC3339 UTC ISO8601 string
// e.g. "2024-10-04T00:00:00.0000000Z"
func FormatEmbyDate(dateStr string) string {
	dateStr = strings.TrimSpace(dateStr)
	if dateStr == "" {
		return ""
	}
	dateStr = strings.ReplaceAll(dateStr, "/", "-")

	if t, err := time.Parse("2006-01-02", dateStr); err == nil {
		return t.UTC().Format("2006-01-02T15:04:05.0000000Z")
	}
	if t, err := time.Parse("2006-01", dateStr); err == nil {
		return t.UTC().Format("2006-01-02T15:04:05.0000000Z")
	}
	if t, err := time.Parse("2006", dateStr); err == nil {
		return t.UTC().Format("2006-01-02T15:04:05.0000000Z")
	}
	if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
		return t.UTC().Format("2006-01-02T15:04:05.0000000Z")
	}
	return ""
}

func ExtractYear(dateStr string) int {
	dateStr = strings.TrimSpace(dateStr)
	if len(dateStr) >= 4 {
		if y, err := strconv.Atoi(dateStr[:4]); err == nil && y >= 1950 && y <= 2100 {
			return y
		}
	}
	return 0
}

// IsEpisodeAired checks whether an episode has already broadcasted / aired and is likely available.
func IsEpisodeAired(ep *bangumi.BangumiEpisode, sub *bangumi.BangumiSubject) bool {
	if ep == nil {
		return false
	}
	// Filter out non-episode entries (e.g. PV, OP/ED preview, MAD). Type 0=regular, Type 1=SP
	if ep.Type > 1 {
		return false
	}

	cst := time.FixedZone("CST", 8*3600)
	now := time.Now().In(cst)
	todayStr := now.Format("2006-01-02")
	currentHour := now.Hour()

	epDate := strings.TrimSpace(ep.AirDate)
	if epDate != "" {
		epDate = strings.ReplaceAll(epDate, "/", "-")
		if len(epDate) >= 10 {
			d := epDate[:10]
			if d < todayStr {
				return true
			}
			if d > todayStr {
				return false
			}
			// Broadcast on today's date: anime broadcasts in Japan late evening (22:00~24:00 CST)
			// and web resources only become available after ~22:00 CST
			return currentHour >= 22
		}
		if len(epDate) == 7 { // YYYY-MM
			currentMonth := now.Format("2006-01")
			return epDate <= currentMonth
		}
		if len(epDate) == 4 { // YYYY
			currentYear := now.Format("2006")
			return epDate <= currentYear
		}
		if t, err := time.Parse(time.RFC3339, epDate); err == nil {
			tDate := t.In(cst).Format("2006-01-02")
			if tDate < todayStr {
				return true
			}
			if tDate > todayStr {
				return false
			}
			return currentHour >= 22
		}
	}

	// If episode has no explicit air date, inspect subject air date
	if sub != nil {
		subDate := strings.TrimSpace(sub.AirDate)
		if subDate != "" {
			subDate = strings.ReplaceAll(subDate, "/", "-")
			if len(subDate) >= 10 {
				if st, err := time.Parse("2006-01-02", subDate[:10]); err == nil {
					stInCST := st.In(cst)
					subDateStr := stInCST.Format("2006-01-02")
					if subDateStr > todayStr {
						return false
					}
					// If anime started more than 180 days ago, all standard episodes have aired
					if now.Sub(stInCST) > 180*24*time.Hour {
						return true
					}
					// For current season anime: estimate episode broadcast date: subDate + (epIndex - 1) * 7 days
					epIndex := ep.Ep
					if epIndex <= 0 {
						epIndex = int(ep.Sort)
					}
					if epIndex > 0 {
						estimatedAir := stInCST.Add(time.Duration(epIndex-1) * 7 * 24 * time.Hour)
						estimatedDateStr := estimatedAir.Format("2006-01-02")
						if estimatedDateStr < todayStr {
							return true
						}
						if estimatedDateStr > todayStr {
							return false
						}
						return currentHour >= 22
					}
				}
			}
		}
	}

	// Fallback: allow episode 1 or single-episode subjects, hide speculative future episodes
	epIndex := ep.Ep
	if epIndex <= 0 {
		epIndex = int(ep.Sort)
	}
	return epIndex <= 1
}

// CreateVirtualViews returns the default library roots as UserView for /Users/:id/Views
func CreateVirtualViews(serverId string) []model.BaseItemDto {
	return []model.BaseItemDto{
		{
			Name:                     "每日放送",
			Id:                       ViewIdSchedule,
			Guid:                     ViewIdSchedule,
			ServerId:                 serverId,
			Type:                     "UserView",
			CollectionType:           "tvshows",
			LocationType:             "FileSystem",
			Path:                     "/media/每日放送",
			IsFolder:                 true,
			SortName:                 "每日放送",
			SortIndexNumber:          1,
			ChildCount:               100,
			RecursiveItemCount:       100,
			PrimaryImageAspectRatio:  1.0,
			ImageTags:                map[string]string{"Primary": "schedule_tag"},
			BackdropImageTags:        []string{},
			Overview:                 "Bangumi 每日更新新番放送时间表",
			DisplayPreferencesId:     ViewIdSchedule,
			PresentationUniqueKey:    ViewIdSchedule,
			PlayAccess:               "Full",
			EnableMediaSourceDisplay: true,
			UserData: &model.UserItemDataDto{
				Key:        ViewIdSchedule,
				IsFavorite: false,
				Played:     false,
			},
		},
		{
			Name:                     "热门排行",
			Id:                       ViewIdTrending,
			Guid:                     ViewIdTrending,
			ServerId:                 serverId,
			Type:                     "UserView",
			CollectionType:           "tvshows",
			LocationType:             "FileSystem",
			Path:                     "/media/热门排行",
			IsFolder:                 true,
			SortName:                 "热门排行",
			SortIndexNumber:          2,
			ChildCount:               100,
			RecursiveItemCount:       100,
			PrimaryImageAspectRatio:  1.0,
			ImageTags:                map[string]string{"Primary": "trending_tag"},
			BackdropImageTags:        []string{},
			Overview:                 "当前最受关注的热门番剧排行榜",
			DisplayPreferencesId:     ViewIdTrending,
			PresentationUniqueKey:    ViewIdTrending,
			PlayAccess:               "Full",
			EnableMediaSourceDisplay: true,
			UserData: &model.UserItemDataDto{
				Key:        ViewIdTrending,
				IsFavorite: false,
				Played:     false,
			},
		},
		{
			Name:                     "正在追番",
			Id:                       ViewIdWatching,
			Guid:                     ViewIdWatching,
			ServerId:                 serverId,
			Type:                     "UserView",
			CollectionType:           "tvshows",
			LocationType:             "FileSystem",
			Path:                     "/media/正在追番",
			IsFolder:                 true,
			SortName:                 "正在追番",
			SortIndexNumber:          3,
			ChildCount:               100,
			RecursiveItemCount:       100,
			PrimaryImageAspectRatio:  1.0,
			ImageTags:                map[string]string{"Primary": "watching_tag"},
			BackdropImageTags:        []string{},
			Overview:                 "我正在观看和追更的动漫番剧",
			DisplayPreferencesId:     ViewIdWatching,
			PresentationUniqueKey:    ViewIdWatching,
			PlayAccess:               "Full",
			EnableMediaSourceDisplay: true,
			UserData: &model.UserItemDataDto{
				Key:        ViewIdWatching,
				IsFavorite: false,
				Played:     false,
			},
		},
	}
}

// CreateCollectionFolders returns views for /Library/MediaFolders
func CreateCollectionFolders(serverId string) []model.BaseItemDto {
	return []model.BaseItemDto{
		{
			Name:                     "每日放送",
			Id:                       ViewIdSchedule,
			Guid:                     ViewIdSchedule,
			ServerId:                 serverId,
			Type:                     "CollectionFolder",
			CollectionType:           "tvshows",
			LocationType:             "FileSystem",
			Path:                     "/media/每日放送",
			IsFolder:                 true,
			SortName:                 "每日放送",
			SortIndexNumber:          1,
			PrimaryImageAspectRatio:  1.0,
			ImageTags:                map[string]string{"Primary": "schedule_tag"},
			BackdropImageTags:        []string{},
			Overview:                 "Bangumi 每日更新新番放送时间表",
			DisplayPreferencesId:     ViewIdSchedule,
			PresentationUniqueKey:    ViewIdSchedule,
			PlayAccess:               "Full",
			EnableMediaSourceDisplay: true,
		},
		{
			Name:                     "热门排行",
			Id:                       ViewIdTrending,
			Guid:                     ViewIdTrending,
			ServerId:                 serverId,
			Type:                     "CollectionFolder",
			CollectionType:           "tvshows",
			LocationType:             "FileSystem",
			Path:                     "/media/热门排行",
			IsFolder:                 true,
			SortName:                 "热门排行",
			SortIndexNumber:          2,
			PrimaryImageAspectRatio:  1.0,
			ImageTags:                map[string]string{"Primary": "trending_tag"},
			BackdropImageTags:        []string{},
			Overview:                 "当前最受关注的热门番剧排行榜",
			DisplayPreferencesId:     ViewIdTrending,
			PresentationUniqueKey:    ViewIdTrending,
			PlayAccess:               "Full",
			EnableMediaSourceDisplay: true,
		},
		{
			Name:                     "正在追番",
			Id:                       ViewIdWatching,
			Guid:                     ViewIdWatching,
			ServerId:                 serverId,
			Type:                     "CollectionFolder",
			CollectionType:           "tvshows",
			LocationType:             "FileSystem",
			Path:                     "/media/正在追番",
			IsFolder:                 true,
			SortName:                 "正在追番",
			SortIndexNumber:          3,
			PrimaryImageAspectRatio:  1.0,
			ImageTags:                map[string]string{"Primary": "watching_tag"},
			BackdropImageTags:        []string{},
			Overview:                 "我正在观看和追更的动漫番剧",
			DisplayPreferencesId:     ViewIdWatching,
			PresentationUniqueKey:    ViewIdWatching,
			PlayAccess:               "Full",
			EnableMediaSourceDisplay: true,
		},
	}
}

// SubjectToSeries converts BangumiSubject to Emby Series BaseItemDto
func SubjectToSeries(sub *bangumi.BangumiSubject, serverId string) model.BaseItemDto {
	seriesId := fmt.Sprintf("bgm_sub_%d", sub.Id)
	displayName := sub.GetDisplayName()

	var tags []string
	var genres []string
	for _, t := range sub.Tags {
		tags = append(tags, t.Name)
		if len(genres) < 5 {
			genres = append(genres, t.Name)
		}
	}

	year := ExtractYear(sub.AirDate)
	if year == 0 && sub.AirWeekday > 0 {
		year = time.Now().Year()
	}
	premiereDate := FormatEmbyDate(sub.AirDate)
	if premiereDate == "" && year > 0 {
		premiereDate = FormatEmbyDate(fmt.Sprintf("%d-01-01", year))
	}
	if premiereDate == "" {
		premiereDate = time.Now().UTC().Format("2006-01-02T15:04:05.0000000Z")
	}

	return model.BaseItemDto{
		Name:                    displayName,
		OriginalTitle:           sub.Name,
		ServerId:                serverId,
		Id:                      seriesId,
		Guid:                    seriesId,
		Type:                    "Series",
		LocationType:            "FileSystem",
		Path:                    fmt.Sprintf("/media/Series/%d", sub.Id),
		Overview:                sub.Summary,
		ProductionYear:          year,
		PremiereDate:            premiereDate,
		DateCreated:             premiereDate,
		EndDate:                 premiereDate,
		Status:                  "Continuing",
		SortName:                displayName,
		OfficialRating:          "TV-14",
		ChildCount:              1,
		RecursiveItemCount:      sub.TotalEps,
		CommunityRating:         sub.RatingScore,
		Genres:                  genres,
		Tags:                    tags,
		PrimaryImageAspectRatio: 0.7,
		DisplayPreferencesId:    seriesId,
		PresentationUniqueKey:   seriesId,
		ProviderIds: map[string]string{
			"Bangumi": strconv.Itoa(sub.Id),
		},
		ImageTags: map[string]string{
			"Primary": "primary_tag",
		},
		BackdropImageTags: []string{},
		IsFolder:          true,
		CanDownload:       false,
		SupportsSync:      false,
		UserData: &model.UserItemDataDto{
			Played: false,
		},
	}
}

// SubjectToSeason creates Season 1 for a series
func SubjectToSeason(sub *bangumi.BangumiSubject, serverId string) model.BaseItemDto {
	seriesId := fmt.Sprintf("bgm_sub_%d", sub.Id)
	seasonId := fmt.Sprintf("bgm_season_%d_1", sub.Id)

	year := ExtractYear(sub.AirDate)
	if year == 0 {
		year = time.Now().Year()
	}
	premiereDate := FormatEmbyDate(sub.AirDate)
	if premiereDate == "" && year > 0 {
		premiereDate = FormatEmbyDate(fmt.Sprintf("%d-01-01", year))
	}
	if premiereDate == "" {
		premiereDate = time.Now().UTC().Format("2006-01-02T15:04:05.0000000Z")
	}

	return model.BaseItemDto{
		Name:                    "第 1 季",
		ServerId:                serverId,
		Id:                      seasonId,
		Guid:                    seasonId,
		Type:                    "Season",
		LocationType:            "FileSystem",
		Path:                    fmt.Sprintf("/media/Series/%d/Season 1", sub.Id),
		SeriesId:                seriesId,
		SeriesName:              sub.GetDisplayName(),
		IndexNumber:             1,
		IsFolder:                true,
		ProductionYear:          year,
		PremiereDate:            premiereDate,
		DateCreated:             premiereDate,
		EndDate:                 premiereDate,
		SortName:                "第 1 季",
		PrimaryImageAspectRatio: 0.7,
		DisplayPreferencesId:    seasonId,
		PresentationUniqueKey:   seasonId,
		ProviderIds: map[string]string{
			"Bangumi": strconv.Itoa(sub.Id),
		},
		ImageTags: map[string]string{
			"Primary": "primary_tag",
		},
		BackdropImageTags: []string{},
	}
}

// ParseDurationToTicks converts duration strings (e.g. "23:45", "00:24:10", "24m", "1440") into Emby 100ns ticks.
func ParseDurationToTicks(durationStr string) int64 {
	durationStr = strings.TrimSpace(durationStr)
	if durationStr == "" || durationStr == "0" {
		return 0
	}

	// 1. HH:MM:SS or MM:SS format
	if strings.Contains(durationStr, ":") {
		parts := strings.Split(durationStr, ":")
		var totalSec int64
		if len(parts) == 3 {
			h, _ := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
			m, _ := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
			s, _ := strconv.ParseInt(strings.TrimSpace(parts[2]), 10, 64)
			totalSec = h*3600 + m*60 + s
		} else if len(parts) == 2 {
			m, _ := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
			s, _ := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
			totalSec = m*60 + s
		}
		if totalSec > 0 {
			return totalSec * 10000000
		}
	}

	// 2. "24m", "23m45s", "1420s"
	if d, err := time.ParseDuration(durationStr); err == nil && d > 0 {
		return d.Nanoseconds() / 100
	}

	// 3. Pure number (seconds or minutes)
	if sec, err := strconv.ParseInt(durationStr, 10, 64); err == nil && sec > 0 {
		if sec <= 180 {
			// If number <= 180 and no unit, interpret as minutes (e.g. "24" -> 24 minutes)
			return sec * 60 * 10000000
		}
		return sec * 10000000
	}

	return 0
}

// CalculateEpisodeTicks determines the most accurate duration for an episode in ticks.
func CalculateEpisodeTicks(ep *bangumi.BangumiEpisode, sub *bangumi.BangumiSubject) int64 {
	// 1. Direct Bangumi episode duration field
	if ep != nil && ep.Duration != "" {
		if ticks := ParseDurationToTicks(ep.Duration); ticks > 0 {
			return ticks
		}
	}

	// 2. Special / SP episode
	if ep != nil && ep.Type == 1 {
		return 3 * 60 * 10000000 // 3 minutes for SP / Specials
	}

	// 3. Subject-level heuristics (Movie / Short anime)
	if sub != nil {
		name := strings.ToLower(sub.GetDisplayName() + " " + sub.Name)
		isMovie := sub.TotalEps == 1 && (strings.Contains(name, "剧场版") || strings.Contains(name, "movie") || strings.Contains(name, "电影") || strings.Contains(name, "ova"))
		isShort := false

		for _, tag := range sub.Tags {
			tName := strings.ToLower(tag.Name)
			if strings.Contains(tName, "剧场版") || strings.Contains(tName, "电影") {
				isMovie = true
			}
			if strings.Contains(tName, "泡面番") || strings.Contains(tName, "动态漫画") || strings.Contains(tName, "短片") || strings.Contains(tName, "pv") {
				isShort = true
			}
		}

		if isMovie {
			return 95 * 60 * 10000000 // ~95 minutes
		}
		if isShort {
			return 4 * 60 * 10000000 // ~4 minutes
		}
	}

	// Standard TV episode default estimate: 24 minutes
	return 24 * 60 * 10000000
}

// EpisodeToEmbyEpisode converts BangumiEpisode to Emby Episode BaseItemDto
func EpisodeToEmbyEpisode(ep *bangumi.BangumiEpisode, sub *bangumi.BangumiSubject, serverId string) model.BaseItemDto {
	seriesId := fmt.Sprintf("bgm_sub_%d", sub.Id)
	seasonId := fmt.Sprintf("bgm_season_%d_1", sub.Id)
	epIndex := ep.Ep
	if epIndex <= 0 {
		epIndex = int(ep.Sort)
	}
	if epIndex <= 0 {
		epIndex = 1
	}
	// Encode both subjectId and episodeIndex so we can always decode both from itemId!
	episodeId := fmt.Sprintf("bgm_ep_%d_%d_%d", sub.Id, epIndex, ep.Id)

	epTitle := ep.NameCn
	if epTitle == "" {
		epTitle = ep.Name
	}
	if epTitle == "" {
		epTitle = fmt.Sprintf("第 %d 集", epIndex)
	}

	seasonIndex := 1
	if ep.Type == 1 { // SP
		seasonIndex = 0
		seasonId = fmt.Sprintf("bgm_season_%d_0", sub.Id)
	}

	epDate := ep.AirDate
	if epDate == "" {
		epDate = sub.AirDate
	}
	year := ExtractYear(epDate)
	if year == 0 {
		year = ExtractYear(sub.AirDate)
	}
	if year == 0 {
		year = time.Now().Year()
	}
	premiereDate := FormatEmbyDate(epDate)
	if premiereDate == "" {
		premiereDate = FormatEmbyDate(sub.AirDate)
	}
	if premiereDate == "" {
		premiereDate = FormatEmbyDate(fmt.Sprintf("%d-01-01", year))
	}
	if premiereDate == "" {
		premiereDate = time.Now().UTC().Format("2006-01-02T15:04:05.0000000Z")
	}

	runTimeTicks := CalculateEpisodeTicks(ep, sub)

	defaultStreams := []model.MediaStream{
		{
			Type:      "Video",
			Index:     0,
			Codec:     "h264",
			Width:     1920,
			Height:    1080,
			BitRate:   4000000,
			IsAVC:     true,
			IsDefault: true,
		},
		{
			Type:      "Audio",
			Index:     1,
			Codec:     "aac",
			Language:  "jpn",
			IsDefault: true,
		},
		{
			Type:           "Subtitle",
			Index:          2,
			Codec:          "ass",
			Language:       "chi",
			DisplayTitle:   "弹弹play 弹幕 (ASS)",
			IsExternal:     true,
			DeliveryMethod: "External",
			DeliveryUrl:    fmt.Sprintf("/emby/Videos/%s/subtitles/0/Stream.ass", episodeId),
			IsDefault:      true,
		},
	}

	mediaSource := model.MediaSourceInfo{
		Id:                   episodeId,
		Name:                 "Akari Stream",
		Type:                 "Default",
		Container:            "mp4",
		Protocol:             "Http",
		Path:                 fmt.Sprintf("/stream/m3u8?item_id=%s", episodeId),
		Bitrate:              4000000,
		RunTimeTicks:         runTimeTicks,
		SupportsDirectPlay:   true,
		SupportsDirectStream: true,
		SupportsTranscoding:  false,
		IsRemote:             true,
		MediaStreams:         defaultStreams,
	}

	return model.BaseItemDto{
		Name:                    epTitle,
		OriginalTitle:           ep.Name,
		ServerId:                serverId,
		Id:                      episodeId,
		Guid:                    episodeId,
		Type:                    "Episode",
		LocationType:            "FileSystem",
		MediaType:               "Video",
		Container:               "mp4",
		Path:                    fmt.Sprintf("/stream/m3u8?item_id=%s", episodeId),
		MediaSources:            []model.MediaSourceInfo{mediaSource},
		MediaStreams:            defaultStreams,
		PlayAccess:              "Full",
		CanPlay:                 true,
		CanResume:               true,
		IsPlaceHolder:           false,
		SeriesId:                seriesId,
		SeriesName:              sub.GetDisplayName(),
		SeasonId:                seasonId,
		SeasonName:              fmt.Sprintf("第 %d 季", seasonIndex),
		IndexNumber:             epIndex,
		ParentIndexNumber:       seasonIndex,
		Overview:                ep.Description,
		ProductionYear:          year,
		PremiereDate:            premiereDate,
		DateCreated:             premiereDate,
		EndDate:                 premiereDate,
		SortName:                epTitle,
		OfficialRating:          "TV-14",
		RunTimeTicks:            runTimeTicks,
		PrimaryImageAspectRatio: 1.7777777777777777,
		DisplayPreferencesId:    episodeId,
		PresentationUniqueKey:   episodeId,
		ProviderIds: map[string]string{
			"Bangumi":   strconv.Itoa(sub.Id),
			"BangumiEp": strconv.Itoa(ep.Id),
		},
		ImageTags: map[string]string{
			"Primary": "primary_tag",
		},
		BackdropImageTags: []string{},
		IsFolder:          false,
		UserData: &model.UserItemDataDto{
			Played: false,
		},
	}
}

func ExtractSubjectId(id string) int {
	clean := strings.TrimPrefix(id, "bgm_sub_")
	clean = strings.TrimPrefix(clean, "bgm_season_")
	clean = strings.TrimPrefix(clean, "bgm_ep_")
	parts := strings.Split(clean, "_")
	if len(parts) > 0 {
		if sid, err := strconv.Atoi(parts[0]); err == nil {
			return sid
		}
	}
	return 0
}

func ExtractEpisodeInfo(id string) (subjectId int, epIndex int) {
	if !strings.HasPrefix(id, "bgm_ep_") {
		return ExtractSubjectId(id), 1
	}
	clean := strings.TrimPrefix(id, "bgm_ep_")
	parts := strings.Split(clean, "_")
	if len(parts) >= 2 {
		sid, _ := strconv.Atoi(parts[0])
		idx, _ := strconv.Atoi(parts[1])
		return sid, idx
	}
	// Fallback single-part ID
	sid, _ := strconv.Atoi(parts[0])
	return sid, 1
}

func ExtractEpisodeId(id string) int {
	if !strings.HasPrefix(id, "bgm_ep_") {
		return 0
	}
	clean := strings.TrimPrefix(id, "bgm_ep_")
	parts := strings.Split(clean, "_")
	if len(parts) >= 3 {
		if epId, err := strconv.Atoi(parts[2]); err == nil {
			return epId
		}
	}
	return 0
}
