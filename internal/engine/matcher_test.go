package engine

import (
	"testing"
)

func TestScoreCandidate(t *testing.T) {
	tests := []struct {
		target    string
		candidate string
		minScore  int
		maxScore  int
	}{
		// 1. MyGO vs S1 vs S2
		{"BanG Dream! It's MyGO!!!!!", "BanG Dream! It's MyGO!!!!!", 100, 100},
		{"BanG Dream! It's MyGO!!!!!", "BanG Dream! Its MyGO!!!!!", 90, 100},
		{"BanG Dream! It's MyGO!!!!!", "BanG Dream!", 0, 30},
		{"BanG Dream! It's MyGO!!!!!", "BanG Dream! 第二季", 0, 30},
		{"BanG Dream! It's MyGO!!!!!", "BanG Dream! Ave Mujica", 0, 30},

		// 2. 药屋少女的呢喃 第三季 vs S1 vs S2
		{"药屋少女的呢喃 第三季", "药屋少女的呢喃第三季", 90, 100},
		{"药屋少女的呢喃 第三季", "药屋少女的呢喃", 0, 30},
		{"药屋少女的呢喃 第三季", "药屋少女的呢喃第二季", 0, 30},

		// 3. 侦探已经死了。 第二季 vs 侦探已死第二季
		{"侦探已经死了。 第二季", "侦探已死第二季", 80, 100},
		{"侦探已经死了。 第二季", "侦探已死", 0, 30},
		{"侦探已经死了。 第二季", "侦探已经死了。", 0, 30},

		// 4. FX战士久留美
		{"FX战士久留美", "FX战士久留美", 100, 100},
		{"FX战士久留美", "FX 战士久留美", 95, 100},
	}

	for _, tt := range tests {
		score := ScoreCandidate(tt.target, tt.candidate)
		t.Logf("Score ['%s' vs '%s'] = %d", tt.target, tt.candidate, score)
		if score < tt.minScore || score > tt.maxScore {
			t.Errorf("Score ['%s' vs '%s'] = %d; expected [%d, %d]",
				tt.target, tt.candidate, score, tt.minScore, tt.maxScore)
		}
	}
}

func TestExtractEpisodeNumber(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		found    bool
	}{
		{"第01集", 1, true},
		{"第1集", 1, true},
		{"1", 1, true},
		{"01", 1, true},
		{"第02集", 2, true},
		{"2", 2, true},
		{"第10集", 10, true},
		{"10", 10, true},
		{"第12集", 12, true},
		{"12", 12, true},
		{"49", 49, true},
		{"第49集", 49, true},
		{"PV1", 0, false},
		{"预告片", 0, false},
	}

	for _, tt := range tests {
		n, ok := ExtractEpisodeNumber(tt.input)
		if ok != tt.found || n != tt.expected {
			t.Errorf("ExtractEpisodeNumber(%q) = (%d, %v); expected (%d, %v)",
				tt.input, n, ok, tt.expected, tt.found)
		}
	}
}

func TestGenerateSearchQueries(t *testing.T) {
	queries := GenerateSearchQueries("侦探已经死了。 第二季", "探偵はもう、死んでいる。 Season 2", "侦探已经死了。 第二季")
	t.Logf("Queries for 侦探已经死了。 第二季: %v", queries)

	hasZtYs2 := false
	hasBase := false
	for _, q := range queries {
		if q == "侦探已死第二季" || q == "侦探已死 第二季" {
			hasZtYs2 = true
		}
		if q == "侦探已经死了" || q == "侦探已死" {
			hasBase = true
		}
	}
	if !hasZtYs2 {
		t.Errorf("Expected queries to contain 侦探已死第二季")
	}
	if !hasBase {
		t.Errorf("Expected queries to contain base keyword 侦探已经死了 or 侦探已死")
	}

	mygoQueries := GenerateSearchQueries("BanG Dream! It's MyGO!!!!!", "BanG Dream! It's MyGO!!!!!", "BanG Dream! It's MyGO!!!!!")
	t.Logf("Queries for MyGO: %v", mygoQueries)
	hasMyGO := false
	for _, q := range mygoQueries {
		if q == "MyGO" {
			hasMyGO = true
		}
	}
	if !hasMyGO {
		t.Errorf("Expected MyGO queries to contain 'MyGO'")
	}
}
