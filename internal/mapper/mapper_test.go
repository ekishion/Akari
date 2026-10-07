package mapper

import (
	"testing"
	"time"

	"akari-bridge/internal/bangumi"
)

func TestIsEpisodeAired(t *testing.T) {
	cst := time.FixedZone("CST", 8*3600)
	now := time.Now().In(cst)
	pastDate := now.Add(-7 * 24 * time.Hour).Format("2006-01-02")
	futureDate := now.Add(7 * 24 * time.Hour).Format("2006-01-02")
	todayDate := now.Format("2006-01-02")

	sub := &bangumi.BangumiSubject{
		Id:      12345,
		Name:    "Test Anime",
		AirDate: pastDate,
	}

	epPast := &bangumi.BangumiEpisode{
		Id:      1,
		Ep:      1,
		Type:    0,
		AirDate: pastDate,
	}
	if !IsEpisodeAired(epPast, sub) {
		t.Errorf("expected past episode to be aired")
	}

	epFuture := &bangumi.BangumiEpisode{
		Id:      2,
		Ep:      2,
		Type:    0,
		AirDate: futureDate,
	}
	if IsEpisodeAired(epFuture, sub) {
		t.Errorf("expected future episode to NOT be aired")
	}

	// PV / OP / ED filtering (Type > 1)
	epPV := &bangumi.BangumiEpisode{
		Id:      3,
		Ep:      0,
		Type:    4, // PV
		AirDate: pastDate,
	}
	if IsEpisodeAired(epPV, sub) {
		t.Errorf("expected PV (Type 4) to NOT be aired as playable episode")
	}

	// Today's episode
	epToday := &bangumi.BangumiEpisode{
		Id:      4,
		Ep:      2,
		Type:    0,
		AirDate: todayDate,
	}
	expectedToday := now.Hour() >= 22
	if IsEpisodeAired(epToday, sub) != expectedToday {
		t.Errorf("expected today episode aired=%v for hour=%d, got %v", expectedToday, now.Hour(), IsEpisodeAired(epToday, sub))
	}

	// Completed older series with empty ep airdate
	oldSub := &bangumi.BangumiSubject{
		Id:      999,
		AirDate: "2020-01-01",
	}
	epOld := &bangumi.BangumiEpisode{
		Id:      10,
		Ep:      10,
		Type:    0,
		AirDate: "",
	}
	if !IsEpisodeAired(epOld, oldSub) {
		t.Errorf("expected old completed series episode to be aired")
	}
}

func TestFormatEmbyDate(t *testing.T) {
	d := FormatEmbyDate("2026-10-08")
	if d != "2026-10-08T00:00:00.0000000Z" {
		t.Errorf("unexpected date format: %s", d)
	}

	year := ExtractYear("2026-10-08")
	if year != 2026 {
		t.Errorf("unexpected year: %d", year)
	}
}

func TestParseDurationToTicks(t *testing.T) {
	// "23:45" -> 1425s = 14,250,000,000 ticks
	if ticks := ParseDurationToTicks("23:45"); ticks != 1425*10000000 {
		t.Errorf("expected 14250000000 ticks for 23:45, got %d", ticks)
	}

	// "00:24:10" -> 1450s
	if ticks := ParseDurationToTicks("00:24:10"); ticks != 1450*10000000 {
		t.Errorf("expected 14500000000 ticks for 00:24:10, got %d", ticks)
	}

	// "01:45:00" -> 6300s
	if ticks := ParseDurationToTicks("01:45:00"); ticks != 6300*10000000 {
		t.Errorf("expected 63000000000 ticks for 01:45:00, got %d", ticks)
	}

	// "24m" -> 1440s
	if ticks := ParseDurationToTicks("24m"); ticks != 1440*10000000 {
		t.Errorf("expected 14400000000 ticks for 24m, got %d", ticks)
	}

	// "1420" -> 1420s
	if ticks := ParseDurationToTicks("1420"); ticks != 1420*10000000 {
		t.Errorf("expected 14200000000 ticks for 1420, got %d", ticks)
	}

	// "" -> 0
	if ticks := ParseDurationToTicks(""); ticks != 0 {
		t.Errorf("expected 0 for empty duration, got %d", ticks)
	}
}

func TestCalculateEpisodeTicks(t *testing.T) {
	// SP episode
	epSP := &bangumi.BangumiEpisode{Id: 1, Type: 1}
	if ticks := CalculateEpisodeTicks(epSP, nil); ticks != 3*60*10000000 {
		t.Errorf("expected 3 mins for SP, got %d", ticks)
	}

	// Movie subject
	subMovie := &bangumi.BangumiSubject{
		Id:       2,
		Name:     "K-ON! 剧场版",
		TotalEps: 1,
		Tags:     []bangumi.SubjectTag{{Name: "剧场版"}},
	}
	epNormal := &bangumi.BangumiEpisode{Id: 2, Ep: 1}
	if ticks := CalculateEpisodeTicks(epNormal, subMovie); ticks != 95*60*10000000 {
		t.Errorf("expected 95 mins for Movie, got %d", ticks)
	}

	// Short anime (泡面番)
	subShort := &bangumi.BangumiSubject{
		Id:   3,
		Name: "黑塔利亚",
		Tags: []bangumi.SubjectTag{{Name: "泡面番"}},
	}
	if ticks := CalculateEpisodeTicks(epNormal, subShort); ticks != 4*60*10000000 {
		t.Errorf("expected 4 mins for Short anime, got %d", ticks)
	}

	// Standard TV
	subTV := &bangumi.BangumiSubject{Id: 4, Name: "普通新番", TotalEps: 12}
	if ticks := CalculateEpisodeTicks(epNormal, subTV); ticks != 24*60*10000000 {
		t.Errorf("expected 24 mins for TV, got %d", ticks)
	}
}
