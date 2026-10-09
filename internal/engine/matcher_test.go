package engine

import (
	"testing"
)

func TestScoreCandidate(t *testing.T) {
	tests := []struct {
		target    string
		candidate string
		isMovie   bool
		minScore  int
		maxScore  int
	}{
		// 1. MyGO vs S1 vs S2
		{"BanG Dream! It's MyGO!!!!!", "BanG Dream! It's MyGO!!!!!", false, 100, 100},
		{"BanG Dream! It's MyGO!!!!!", "BanG Dream! Its MyGO!!!!!", false, 90, 100},
		{"BanG Dream! It's MyGO!!!!!", "BanG Dream!", false, 0, 30},
		{"BanG Dream! It's MyGO!!!!!", "BanG Dream! 第二季", false, 0, 30},
		{"BanG Dream! It's MyGO!!!!!", "BanG Dream! Ave Mujica", false, 0, 30},

		// 2. 药屋少女的呢喃 第三季 vs S1 vs S2
		{"药屋少女的呢喃 第三季", "药屋少女的呢喃第三季", false, 90, 100},
		{"药屋少女的呢喃 第三季", "药屋少女的呢喃", false, 0, 30},
		{"药屋少女的呢喃 第三季", "药屋少女的呢喃第二季", false, 0, 30},

		// 3. 侦探已经死了。 第二季 vs 侦探已死第二季
		{"侦探已经死了。 第二季", "侦探已死第二季", false, 80, 100},
		{"侦探已经死了。 第二季", "侦探已死", false, 0, 30},
		{"侦探已经死了。 第二季", "侦探已经死了。", false, 0, 30},

		// 4. FX战士久留美
		{"FX战士久留美", "FX战士久留美", false, 100, 100},
		{"FX战士久留美", "FX 战士久留美", false, 95, 100},

		// 5. Anime Movies (超辉夜姬！, 剧场版, 电影)
		{"超辉夜姬！", "超辉夜姬！", true, 70, 100},
		{"超辉夜姬！", "超辉夜姬 剧场版", true, 90, 100},
		{"超辉夜姬！", "超辉夜姬 电影", true, 90, 100},
		{"超辉夜姬！", "剧场版 超辉夜姬！", true, 90, 100},
		{"铃芽之旅", "铃芽之旅 电影", true, 90, 100},
		{"鬼灭之刃 无限列车篇", "鬼灭之刃 剧场版 无限列车篇", true, 90, 100},

		// 6. Same-named Movie vs TV Series (e.g. 罗小黑战记)
		{"罗小黑战记", "罗小黑战记 电影版", true, 95, 100},   // Target is Movie -> Movie candidate is 100
		{"罗小黑战记", "罗小黑战记大电影", true, 95, 100},    // Target is Movie -> Movie candidate is 100
		{"罗小黑战记", "罗小黑战记", true, 70, 75},         // Target is Movie -> Plain TV candidate is lowered to 70-75
		{"罗小黑战记", "罗小黑战记", false, 100, 100},       // Target is TV -> Plain TV candidate is 100
		{"罗小黑战记", "罗小黑战记 电影版", false, 0, 30},      // Target is TV -> Movie candidate is rejected <= 30
		{"罗小黑战记", "罗小黑战记大电影", false, 0, 30},     // Target is TV -> Movie candidate is rejected <= 30
	}

	synonyms := map[string]string{
		"已经死了": "已死",
		"超时空":  "超",
	}

	for _, tt := range tests {
		score := ScoreCandidate(tt.target, tt.candidate, tt.isMovie, synonyms)
		t.Logf("Score ['%s' vs '%s', isMovie=%v] = %d", tt.target, tt.candidate, tt.isMovie, score)
		if score < tt.minScore || score > tt.maxScore {
			t.Errorf("Score ['%s' vs '%s', isMovie=%v] = %d; expected [%d, %d]",
				tt.target, tt.candidate, tt.isMovie, score, tt.minScore, tt.maxScore)
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
	synonyms := map[string]string{
		"已经死了": "已死",
		"超时空":  "超",
	}

	queries := GenerateSearchQueries("侦探已经死了。 第二季", "探偵はもう、死んでいる。 Season 2", "侦探已经死了。 第二季", false, synonyms)
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

	mygoQueries := GenerateSearchQueries("BanG Dream! It's MyGO!!!!!", "BanG Dream! It's MyGO!!!!!", "BanG Dream! It's MyGO!!!!!", false, synonyms, "MyGO", "BanG Dream")
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

	movieQueries := GenerateSearchQueries("罗小黑战记", "", "罗小黑战记", true, synonyms)
	t.Logf("Queries for 罗小黑战记 (Movie): %v", movieQueries)
	if len(movieQueries) == 0 || movieQueries[0] != "罗小黑战记 电影版" {
		t.Errorf("Expected movie queries to prioritize '罗小黑战记 电影版', got %v", movieQueries)
	}
}
