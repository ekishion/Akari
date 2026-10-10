package service

import (
	"context"
	"fmt"
	"strings"

	"akari-bridge/internal/bangumi"
	"akari-bridge/internal/config"
	"akari-bridge/internal/domain"
	"akari-bridge/internal/mapper"
	"akari-bridge/internal/model"
)

type CatalogService struct {
	cfg           *config.Config
	bangumiClient *bangumi.Client
	playbackRepo  domain.IPlaybackRepository
	favRepo       domain.IFavoriteRepository
	userRepo      domain.IUserRepository
}

func NewCatalogService(
	cfg *config.Config,
	bangumiClient *bangumi.Client,
	playbackRepo domain.IPlaybackRepository,
	favRepo domain.IFavoriteRepository,
	userRepo domain.IUserRepository,
) *CatalogService {
	return &CatalogService{
		cfg:           cfg,
		bangumiClient: bangumiClient,
		playbackRepo:  playbackRepo,
		favRepo:       favRepo,
		userRepo:      userRepo,
	}
}

func (s *CatalogService) GetViews(userId string) ([]model.BaseItemDto, error) {
	return mapper.CreateVirtualViews(s.cfg.ServerId), nil
}

func (s *CatalogService) GetShows(ctx context.Context, userId string, startIndex, limit int) (*model.QueryResult[model.BaseItemDto], error) {
	if limit <= 0 {
		limit = 50
	}
	subjects, err := s.bangumiClient.GetCalendar()
	if err != nil {
		return &model.QueryResult[model.BaseItemDto]{Items: []model.BaseItemDto{}, TotalRecordCount: 0}, nil
	}

	items := make([]model.BaseItemDto, 0, len(subjects))
	for _, sub := range subjects {
		item := mapper.SubjectToSeries(&sub, s.cfg.ServerId)
		item.UserData = s.getUserItemData(userId, item.Id)
		items = append(items, item)
	}

	total := len(items)
	var paged []model.BaseItemDto
	if startIndex < total {
		end := startIndex + limit
		if end > total {
			end = total
		}
		paged = items[startIndex:end]
	} else {
		paged = []model.BaseItemDto{}
	}

	return &model.QueryResult[model.BaseItemDto]{
		Items:            paged,
		TotalRecordCount: total,
	}, nil
}

func (s *CatalogService) GetItems(ctx context.Context, userId string, parentId string, searchTerm string, filters map[string]string, startIndex, limit int) (*model.QueryResult[model.BaseItemDto], error) {
	if limit <= 0 {
		limit = 50
	}

	// 1. Favorites query
	if filters["IsFavorite"] == "true" || filters["IsFavorite"] == "1" {
		var favItems []model.BaseItemDto
		seen := make(map[string]bool)

		if s.favRepo != nil {
			favIds, _ := s.favRepo.ListFavoriteItems(userId)
			for _, fid := range favIds {
				if strings.HasPrefix(fid, "bgm_sub_") || strings.HasPrefix(fid, "bgm_ep_") || strings.HasPrefix(fid, "bgm_season_") {
					sid := mapper.ExtractSubjectId(fid)
					seriesId := fmt.Sprintf("bgm_sub_%d", sid)
					if sid > 0 && !seen[seriesId] {
						seen[seriesId] = true
						if sub, err := s.bangumiClient.GetSubject(sid); err == nil && sub != nil {
							item := mapper.SubjectToSeries(sub, s.cfg.ServerId)
							item.UserData = s.getUserItemData(userId, seriesId)
							favItems = append(favItems, item)
						}
					}
				}
			}
		}

		total := len(favItems)
		var paged []model.BaseItemDto
		if startIndex < total {
			end := startIndex + limit
			if end > total {
				end = total
			}
			paged = favItems[startIndex:end]
		} else {
			paged = []model.BaseItemDto{}
		}
		return &model.QueryResult[model.BaseItemDto]{Items: paged, TotalRecordCount: total}, nil
	}

	// 2. Search
	if searchTerm != "" {
		subjects, err := s.bangumiClient.Search(searchTerm, limit)
		if err != nil {
			return &model.QueryResult[model.BaseItemDto]{Items: []model.BaseItemDto{}, TotalRecordCount: 0}, nil
		}
		items := make([]model.BaseItemDto, 0, len(subjects))
		for _, sub := range subjects {
			item := mapper.SubjectToSeries(&sub, s.cfg.ServerId)
			item.UserData = s.getUserItemData(userId, item.Id)
			items = append(items, item)
		}
		return &model.QueryResult[model.BaseItemDto]{Items: items, TotalRecordCount: len(items)}, nil
	}

	// 3. Fallback to calendar shows
	return s.GetShows(ctx, userId, startIndex, limit)
}

func (s *CatalogService) GetItem(ctx context.Context, userId, itemId string) (*model.BaseItemDto, error) {
	if strings.HasPrefix(itemId, "bgm_sub_") {
		subId := mapper.ExtractSubjectId(itemId)
		sub, err := s.bangumiClient.GetSubject(subId)
		if err != nil || sub == nil {
			return nil, fmt.Errorf("subject not found: %s", itemId)
		}
		item := mapper.SubjectToSeries(sub, s.cfg.ServerId)
		item.UserData = s.getUserItemData(userId, item.Id)
		return &item, nil
	}

	if strings.HasPrefix(itemId, "bgm_season_") {
		subId := mapper.ExtractSubjectId(itemId)
		sub, err := s.bangumiClient.GetSubject(subId)
		if err != nil || sub == nil {
			return nil, fmt.Errorf("season not found: %s", itemId)
		}
		item := mapper.SubjectToSeason(sub, s.cfg.ServerId)
		item.UserData = s.getUserItemData(userId, item.Id)
		return &item, nil
	}

	if strings.HasPrefix(itemId, "bgm_ep_") {
		subId, epIndex := mapper.ExtractEpisodeInfo(itemId)
		epId := mapper.ExtractEpisodeId(itemId)
		sub, err := s.bangumiClient.GetSubject(subId)
		if err != nil || sub == nil {
			return nil, fmt.Errorf("episode subject not found: %s", itemId)
		}

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

		if targetEp != nil {
			item := mapper.EpisodeToEmbyEpisode(targetEp, sub, s.cfg.ServerId)
			item.UserData = s.getUserItemData(userId, item.Id)
			if s.playbackRepo != nil {
				if ticks := s.playbackRepo.GetItemTotalTicks(item.Id); ticks > 0 {
					item.RunTimeTicks = ticks
				}
			}
			return &item, nil
		}
	}

	return nil, fmt.Errorf("unknown item id: %s", itemId)
}

func (s *CatalogService) GetSeasons(ctx context.Context, userId, seriesId string) (*model.QueryResult[model.BaseItemDto], error) {
	subId := mapper.ExtractSubjectId(seriesId)
	sub, err := s.bangumiClient.GetSubject(subId)
	if err != nil || sub == nil {
		return &model.QueryResult[model.BaseItemDto]{Items: []model.BaseItemDto{}, TotalRecordCount: 0}, nil
	}

	season := mapper.SubjectToSeason(sub, s.cfg.ServerId)
	season.UserData = s.getUserItemData(userId, season.Id)
	return &model.QueryResult[model.BaseItemDto]{
		Items:            []model.BaseItemDto{season},
		TotalRecordCount: 1,
	}, nil
}

func (s *CatalogService) GetEpisodes(ctx context.Context, userId, seriesId, seasonId string) (*model.QueryResult[model.BaseItemDto], error) {
	subId := mapper.ExtractSubjectId(seriesId)
	if subId <= 0 && seasonId != "" {
		subId = mapper.ExtractSubjectId(seasonId)
	}

	sub, err := s.bangumiClient.GetSubject(subId)
	if err != nil || sub == nil {
		return &model.QueryResult[model.BaseItemDto]{Items: []model.BaseItemDto{}, TotalRecordCount: 0}, nil
	}

	episodes, err := s.bangumiClient.GetEpisodes(subId)
	if err != nil || len(episodes) == 0 {
		return &model.QueryResult[model.BaseItemDto]{Items: []model.BaseItemDto{}, TotalRecordCount: 0}, nil
	}

	items := make([]model.BaseItemDto, 0, len(episodes))
	for _, ep := range episodes {
		if ep.Type != 0 && ep.Type != 1 {
			continue
		}
		item := mapper.EpisodeToEmbyEpisode(&ep, sub, s.cfg.ServerId)
		item.UserData = s.getUserItemData(userId, item.Id)
		if s.playbackRepo != nil {
			if ticks := s.playbackRepo.GetItemTotalTicks(item.Id); ticks > 0 {
				item.RunTimeTicks = ticks
			}
		}
		items = append(items, item)
	}

	return &model.QueryResult[model.BaseItemDto]{
		Items:            items,
		TotalRecordCount: len(items),
	}, nil
}

func (s *CatalogService) GetNextUp(ctx context.Context, userId, seriesId string) (*model.QueryResult[model.BaseItemDto], error) {
	subId := mapper.ExtractSubjectId(seriesId)
	sub, err := s.bangumiClient.GetSubject(subId)
	if err != nil || sub == nil {
		return &model.QueryResult[model.BaseItemDto]{Items: []model.BaseItemDto{}, TotalRecordCount: 0}, nil
	}

	episodes, err := s.bangumiClient.GetEpisodes(subId)
	if err != nil || len(episodes) == 0 {
		return &model.QueryResult[model.BaseItemDto]{Items: []model.BaseItemDto{}, TotalRecordCount: 0}, nil
	}

	for _, ep := range episodes {
		if ep.Type == 0 || ep.Type == 1 {
			item := mapper.EpisodeToEmbyEpisode(&ep, sub, s.cfg.ServerId)
			ud := s.getUserItemData(userId, item.Id)
			if ud == nil || !ud.Played {
				item.UserData = ud
				return &model.QueryResult[model.BaseItemDto]{
					Items:            []model.BaseItemDto{item},
					TotalRecordCount: 1,
				}, nil
			}
		}
	}

	return &model.QueryResult[model.BaseItemDto]{Items: []model.BaseItemDto{}, TotalRecordCount: 0}, nil
}

func (s *CatalogService) GetSimilarItems(ctx context.Context, userId, itemId string, limit int) (*model.QueryResult[model.BaseItemDto], error) {
	if limit <= 0 {
		limit = 10
	}
	subId := mapper.ExtractSubjectId(itemId)
	if subId <= 0 {
		return &model.QueryResult[model.BaseItemDto]{Items: []model.BaseItemDto{}, TotalRecordCount: 0}, nil
	}

	calendar, err := s.bangumiClient.GetCalendar()
	if err != nil {
		return &model.QueryResult[model.BaseItemDto]{Items: []model.BaseItemDto{}, TotalRecordCount: 0}, nil
	}

	var items []model.BaseItemDto
	for _, sub := range calendar {
		if sub.Id != subId {
			item := mapper.SubjectToSeries(&sub, s.cfg.ServerId)
			item.UserData = s.getUserItemData(userId, item.Id)
			items = append(items, item)
			if len(items) >= limit {
				break
			}
		}
	}

	return &model.QueryResult[model.BaseItemDto]{
		Items:            items,
		TotalRecordCount: len(items),
	}, nil
}

func (s *CatalogService) SetFavorite(userId, itemId string, isFavorite bool) (*model.UserItemDataDto, error) {
	if s.favRepo != nil {
		_ = s.favRepo.ToggleFavoriteItem(userId, itemId, isFavorite)
	}
	return s.getUserItemData(userId, itemId), nil
}

func (s *CatalogService) getUserItemData(userId, itemId string) *model.UserItemDataDto {
	posTicks := int64(0)
	totalTicks := int64(0)
	played := false
	isFav := false

	if s.playbackRepo != nil {
		posTicks, totalTicks, played = s.playbackRepo.GetPlaybackProgress(userId, itemId)
	}
	if s.favRepo != nil {
		isFav = s.favRepo.IsFavoriteItem(userId, itemId)
	}

	playedPct := 0.0
	if totalTicks > 0 {
		playedPct = float64(posTicks) / float64(totalTicks) * 100.0
	}

	return &model.UserItemDataDto{
		Played:                played,
		PlaybackPositionTicks: posTicks,
		PlayedPercentage:     playedPct,
		IsFavorite:           isFav,
		Key:                  itemId,
	}
}
