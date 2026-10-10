package service

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"akari-bridge/internal/config"
	"akari-bridge/internal/domain"
	"akari-bridge/internal/engine"
	"akari-bridge/internal/rules"
)

var appStartTime = time.Now()

type AdminService struct {
	cfg          *config.Config
	ruleMgr      *rules.RuleManager
	eng          *engine.Engine
	activityRepo domain.IActivityRepository
	userRepo     domain.IUserRepository
}

func NewAdminService(
	cfg *config.Config,
	ruleMgr *rules.RuleManager,
	eng *engine.Engine,
	activityRepo domain.IActivityRepository,
	userRepo domain.IUserRepository,
) *AdminService {
	return &AdminService{
		cfg:          cfg,
		ruleMgr:      ruleMgr,
		eng:          eng,
		activityRepo: activityRepo,
		userRepo:     userRepo,
	}
}

func (s *AdminService) GetSystemStatus() (map[string]any, error) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	activeRules := 0
	totalRules := 0
	if s.ruleMgr != nil {
		activeRules = len(s.ruleMgr.GetEnabledPlugins())
		totalRules = len(s.ruleMgr.GetAllPlugins())
	}

	userCount := 1
	if s.userRepo != nil {
		if count, err := s.userRepo.CountUsers(); err == nil && count > 0 {
			userCount = count
		}
	}

	return map[string]any{
		"serverName":     s.cfg.ServerName,
		"serverId":       s.cfg.ServerId,
		"version":        "1.0.0",
		"serverUrl":      s.cfg.GetServerUrl(),
		"httpPort":       s.cfg.HttpPort,
		"udpPort":        s.cfg.UdpPort,
		"uptimeSeconds":  int64(time.Since(appStartTime).Seconds()),
		"activeRules":    activeRules,
		"totalRules":     totalRules,
		"goVersion":      runtime.Version(),
		"os":             runtime.GOOS,
		"arch":           runtime.GOARCH,
		"numGoroutines":  runtime.NumGoroutine(),
		"memoryAllocMB":  m.Alloc / 1024 / 1024,
		"memorySysMB":    m.Sys / 1024 / 1024,
		"singleUserMode": userCount <= 1,
	}, nil
}

func (s *AdminService) GetActivityLogs(limit int) ([]domain.ActivityLog, error) {
	if s.activityRepo == nil {
		return []domain.ActivityLog{}, nil
	}
	return s.activityRepo.GetRecentActivityLogs(limit)
}

func (s *AdminService) LogActivity(logType, action, detail, ip string) error {
	if s.activityRepo == nil {
		return nil
	}
	return s.activityRepo.AddActivityLog(logType, action, detail, ip)
}

func (s *AdminService) TestProxy(testURL string) (bool, string, error) {
	if testURL == "" {
		testURL = "https://www.google.com/generate_204"
	}
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
	}
	start := time.Now()
	resp, err := client.Get(testURL)
	latency := time.Since(start)
	if err != nil {
		return false, fmt.Sprintf("Proxy test failed: %v", err), err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return true, fmt.Sprintf("Proxy reachable (status %d, latency %v)", resp.StatusCode, latency), nil
	}
	return false, fmt.Sprintf("Proxy returned non-2xx status: %d", resp.StatusCode), nil
}

func (s *AdminService) TestRule(ctx context.Context, plugin *engine.Plugin, keyword string) ([]engine.SearchItem, error) {
	if plugin == nil {
		return nil, fmt.Errorf("plugin is nil")
	}
	if keyword == "" {
		keyword = "电锯人"
	}
	return s.eng.Search(ctx, plugin, keyword)
}
