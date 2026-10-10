package service

import (
	"testing"

	"akari-bridge/internal/config"
	"akari-bridge/internal/domain"
	"akari-bridge/internal/engine"
	"akari-bridge/internal/resolver"
)

// mockActivityRepo for unit testing
type mockActivityRepo struct {
	logs []domain.ActivityLog
}

func (m *mockActivityRepo) AddActivityLog(logType, action, detail, ip string) error {
	m.logs = append(m.logs, domain.ActivityLog{Type: logType, Detail: action + ": " + detail, IP: ip})
	return nil
}

func (m *mockActivityRepo) GetRecentActivityLogs(limit int) ([]domain.ActivityLog, error) {
	return m.logs, nil
}

func (m *mockActivityRepo) RecordAuditLog(action, ip, userAgent, details string) error {
	m.logs = append(m.logs, domain.ActivityLog{Action: action, IP: ip, Detail: details})
	return nil
}

func (m *mockActivityRepo) GetAuditLogs(limit, offset int) ([]domain.ActivityLog, int, error) {
	return m.logs, len(m.logs), nil
}

func (m *mockActivityRepo) ClearAuditLogs() error {
	m.logs = nil
	return nil
}

func TestAdminService_GetSystemStatus(t *testing.T) {
	cfg := &config.Config{
		ServerName: "Test Server",
		ServerId:   "test_srv_id",
		HttpPort:   8096,
		UdpPort:    7359,
	}
	mockRepo := &mockActivityRepo{}
	adminSvc := NewAdminService(cfg, nil, nil, mockRepo, nil)

	status, err := adminSvc.GetSystemStatus()
	if err != nil {
		t.Fatalf("GetSystemStatus returned error: %v", err)
	}

	if status["serverName"] != "Test Server" || status["httpPort"] != 8096 {
		t.Fatalf("unexpected system status: %v", status)
	}

	_ = adminSvc.LogActivity("AUTH", "login", "admin logged in", "127.0.0.1")
	logs, _ := adminSvc.GetActivityLogs(10)
	if len(logs) != 1 || logs[0].Type != "AUTH" {
		t.Fatalf("expected 1 activity log, got %v", logs)
	}
}

func TestPlaybackService_AssembleMediaSources(t *testing.T) {
	cfg := &config.Config{
		HttpPort:   8096,
		PublicHost: "127.0.0.1",
	}
	svc := NewPlaybackService(cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	res := &domain.PlaybackResolution{
		RuleStreams: []domain.VerifiedRuleStream{
			{
				Plugin: &engine.Plugin{Name: "baimao"},
				Resolved: &resolver.ResolvedStream{
					RealURL: "https://baimao.com/stream.m3u8",
					Format:  "m3u8",
				},
				Score: 95,
			},
		},
	}

	sources, err := svc.AssembleMediaSources("bgm_ep_13603_1_78326", "http://127.0.0.1:8096", res)
	if err != nil {
		t.Fatalf("AssembleMediaSources returned error: %v", err)
	}

	if len(sources) != 1 {
		t.Fatalf("expected 1 media source, got %d", len(sources))
	}

	if sources[0].Id != "akari_src_baimao_1" || sources[0].Container != "hls" {
		t.Fatalf("unexpected media source: %+v", sources[0])
	}
}
