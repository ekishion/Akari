package bilibili

import (
	"testing"
)

func TestScoreTitleMatch(t *testing.T) {
	s1 := scoreTitleMatch("葬送的芙莉莲", "葬送的芙莉莲", "Sousou no Frieren", false, "葬送的芙莉莲", "Sousou no Frieren", 1, 1)
	if s1 < 0.95 {
		t.Errorf("expected high score for exact match, got %f", s1)
	}

	s2 := scoreTitleMatch("鬼灭之刃 柱训练篇", "鬼灭之刃 柱训练篇", "Kimetsu no Yaiba", false, "鬼灭之刃 柱训练篇", "", 1, 1)
	if s2 < 0.85 {
		t.Errorf("expected high score for matching title, got %f", s2)
	}

	s3 := scoreTitleMatch("BanG Dream! It's MyGO!!!!!", "", "", false, "海贼王", "", 1, 1)
	if s3 > 0.3 {
		t.Errorf("expected low score for unrelated anime, got %f", s3)
	}

	// Season mismatch test: Season 2 requested vs Season 1 candidate -> MUST be 0.0
	s4 := scoreTitleMatch("侦探已经死了。 第二季", "侦探已经死了。 第二季", "探偵はもう、死んでいる。 Season 2", false, "侦探已死", "", 1, 1)
	if s4 != 0.0 {
		t.Errorf("expected 0 score for Season 2 vs Season 1 mismatch, got %f", s4)
	}

	// Season match test: Season 2 requested vs Season 2 candidate -> MUST be >= 0.80
	s5 := scoreTitleMatch("侦探已经死了。 第二季", "侦探已经死了。 第二季", "探偵はもう、死んでいる。 Season 2", false, "侦探已死 第二季", "", 1, 1)
	if s5 < 0.80 {
		t.Errorf("expected high score for Season 2 matching candidate, got %f", s5)
	}

	// Season 1 requested vs Season 2 candidate -> MUST be 0.0
	s6 := scoreTitleMatch("某科学的超电磁炮", "某科学的超电磁炮", "", false, "某科学的超电磁炮 第二季", "", 1, 1)
	if s6 != 0.0 {
		t.Errorf("expected 0 score for Season 1 vs Season 2 candidate mismatch, got %f", s6)
	}

	// Movie test: movie requested vs movie candidate -> high score
	s7 := scoreTitleMatch("罗小黑战记2", "罗小黑战记2", "The Legend of Hei 2", true, "罗小黑战记2", "罗小黑战记2", 2, 2)
	if s7 < 0.90 {
		t.Errorf("expected high score for movie match, got %f", s7)
	}

	// Movie test: movie requested vs TV anime candidate -> must be penalized
	s8 := scoreTitleMatch("罗小黑战记", "罗小黑战记", "", true, "罗小黑战记", "罗小黑战记", 4, 4)
	if s8 > 0.60 {
		t.Errorf("expected low score for movie target matching TV series, got %f", s8)
	}
}

func TestFindTargetEpisode(t *testing.T) {
	eps := []struct {
		ID        int    `json:"id"`
		Aid       int64  `json:"aid"`
		Bvid      string `json:"bvid"`
		Cid       int64  `json:"cid"`
		Title     string `json:"title"`
		LongTitle string `json:"long_title"`
		Badge     string `json:"badge"`
		ShareURL  string `json:"share_url"`
	}{
		{ID: 101, Bvid: "BV1", Cid: 1001, Title: "1", LongTitle: "冒险的终点"},
		{ID: 102, Bvid: "BV2", Cid: 1002, Title: "2", LongTitle: "别无选择的旅行"},
		{ID: 103, Bvid: "BV3", Cid: 1003, Title: "03", LongTitle: "伏拉梅的魔法"},
		{ID: 129, Bvid: "BV29", Cid: 1029, Title: "29", LongTitle: "异界中的灵力"},
	}

	id1, bvid1, cid1, title1 := findTargetEpisode(eps, 1, 1, "冒险的终点", false)
	if id1 != 101 || bvid1 != "BV1" || cid1 != 1001 || title1 != "1 冒险的终点" {
		t.Errorf("unexpected ep 1 match: %d, %s, %d, %s", id1, bvid1, cid1, title1)
	}

	id3, bvid3, cid3, title3 := findTargetEpisode(eps, 3, 3, "", false)
	if id3 != 103 || bvid3 != "BV3" || cid3 != 1003 || title3 != "03 伏拉梅的魔法" {
		t.Errorf("unexpected ep 3 match: %d, %s, %d, %s", id3, bvid3, cid3, title3)
	}

	// Sort episode match test (e.g. epIndex=1, epSort=29 for sub-season arc)
	id29, bvid29, cid29, title29 := findTargetEpisode(eps, 1, 29, "异界中的灵力", false)
	if id29 != 129 || bvid29 != "BV29" || cid29 != 1029 || title29 != "29 异界中的灵力" {
		t.Errorf("unexpected sort ep 29 match: %d, %s, %d, %s", id29, bvid29, cid29, title29)
	}
}

func TestExtractPlayableStream(t *testing.T) {
	res := &PlayURLResponse{}
	res.Data.Quality = 80
	res.Data.Timelength = 1440000
	res.Data.Dash = &DashData{
		Video: []DashStream{
			{ID: 32, BaseURL: "https://upos-sz-mirrorcos.bilivideo.com/v_480p.m4s", Codecs: "avc1.640028", Bandwidth: 500000},
			{ID: 80, BaseURL: "https://upos-sz-mirrorcos.bilivideo.com/v_1080p.m4s", Codecs: "avc1.640028", Bandwidth: 2000000},
			{ID: 116, BaseURL: "https://upos-sz-mirrorcos.bilivideo.com/v_1080p60_av1.m4s", Codecs: "av01.0.08M.10", Bandwidth: 2200000},
		},
		Audio: []DashStream{
			{ID: 30280, BaseURL: "https://upos-sz-mirrorcos.bilivideo.com/a_192k.m4s", Bandwidth: 192000},
		},
	}

	resolved := extractPlayableStream(res, 120)
	if resolved == nil {
		t.Fatalf("expected resolved stream, got nil")
	}
	if !resolved.IsDASH {
		t.Errorf("expected DASH stream")
	}
	// Verify that AVC 1080P is preferred over AV1 1080P60 for compatibility
	if resolved.Codec != "h264" {
		t.Errorf("expected h264 codec, got %s", resolved.Codec)
	}
	if resolved.VideoURL != "https://upos-sz-mirrorcos.bilivideo.com/v_1080p.m4s" {
		t.Errorf("unexpected video URL: %s", resolved.VideoURL)
	}
	if resolved.AudioURL != "https://upos-sz-mirrorcos.bilivideo.com/a_192k.m4s" {
		t.Errorf("unexpected audio URL: %s", resolved.AudioURL)
	}
}

func TestSanitizeUposURL(t *testing.T) {
	mcdnURL := "https://h2i438c.edge.mountaintoys.cn:4483/upgcxcode/36/05/41352760536/test.m4s?e=123"
	backup := []string{"https://upos-sz-mirrorcos.bilivideo.com/upgcxcode/36/05/41352760536/test.m4s?e=123"}
	clean := SanitizeUposURL(mcdnURL, backup)
	if clean != backup[0] {
		t.Errorf("expected backup URL, got %s", clean)
	}

	clean2 := SanitizeUposURL(mcdnURL, nil)
	expected2 := "https://upos-sz-mirrorcos.bilivideo.com/upgcxcode/36/05/41352760536/test.m4s?e=123"
	if clean2 != expected2 {
		t.Errorf("expected rewritten URL %s, got %s", expected2, clean2)
	}
}
