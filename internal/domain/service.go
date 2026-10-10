package domain

import (
	"context"

	"akari-bridge/internal/bilibili"
	"akari-bridge/internal/engine"
	"akari-bridge/internal/model"
	"akari-bridge/internal/resolver"
)

// VerifiedRuleStream is a verified playable stream from a Kazumi third-party rule.
type VerifiedRuleStream struct {
	Plugin   *engine.Plugin            `json:"plugin"`
	Resolved *resolver.ResolvedStream `json:"resolved"`
	Score    int                       `json:"score"`
}

// PlaybackResolution contains the resolved Bilibili stream and/or verified rule streams.
type PlaybackResolution struct {
	BiliStream  *bilibili.ResolvedBiliStream
	RuleStreams []VerifiedRuleStream
}

// IPlaybackService orchestrates multi-source anime episode stream resolving and fallback.
type IPlaybackService interface {
	ResolvePlayback(ctx context.Context, itemId string) (*PlaybackResolution, error)
	AssembleMediaSources(itemId string, proxyBase string, res *PlaybackResolution) ([]model.MediaSourceInfo, error)
	ReportProgress(userId, itemId string, posTicks, totalTicks int64, isPaused bool) error
	ReportStopped(userId, itemId string, posTicks int64) error
}

// ICatalogService manages Bangumi metadata retrieval, library items, and episode listings.
type ICatalogService interface {
	GetViews(userId string) ([]model.BaseItemDto, error)
	GetItems(ctx context.Context, userId string, parentId string, searchTerm string, filters map[string]string, startIndex, limit int) (*model.QueryResult[model.BaseItemDto], error)
	GetItem(ctx context.Context, userId, itemId string) (*model.BaseItemDto, error)
	GetShows(ctx context.Context, userId string, startIndex, limit int) (*model.QueryResult[model.BaseItemDto], error)
	GetSeasons(ctx context.Context, userId, seriesId string) (*model.QueryResult[model.BaseItemDto], error)
	GetEpisodes(ctx context.Context, userId, seriesId, seasonId string) (*model.QueryResult[model.BaseItemDto], error)
	GetNextUp(ctx context.Context, userId, seriesId string) (*model.QueryResult[model.BaseItemDto], error)
	GetSimilarItems(ctx context.Context, userId, itemId string, limit int) (*model.QueryResult[model.BaseItemDto], error)
	SetFavorite(userId, itemId string, isFavorite bool) (*model.UserItemDataDto, error)
}

// IAdminService manages system diagnostics, proxy connectivity, rules, and activity logs.
type IAdminService interface {
	GetSystemStatus() (map[string]any, error)
	GetActivityLogs(limit int) ([]ActivityLog, error)
	LogActivity(logType, action, detail, ip string) error
	TestProxy(testURL string) (bool, string, error)
	TestRule(ctx context.Context, plugin *engine.Plugin, keyword string) ([]engine.SearchItem, error)
}
