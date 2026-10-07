package engine

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	rePunctuation = regexp.MustCompile(`[。！？!?:：·、，,~～『』「」\[\]\(\)\-_—"'\s]+`)
	reAlphaHan    = regexp.MustCompile(`([a-zA-Z0-9]+)([\p{Han}]+)`)
	reEpPattern1  = regexp.MustCompile(`(?:第|话|ep|e)\s*0*([0-9]+)\s*(?:集|话)?`)
	reEpPattern2  = regexp.MustCompile(`0*([0-9]+)\s*(?:集|话)`)
	reEpPattern3  = regexp.MustCompile(`^0*([0-9]+)$`)
	reEpAnyNumber = regexp.MustCompile(`(?:^|[^0-9])0*([0-9]+)(?:[^0-9]|$)`)
)

// ExtractEpisodeNumber extracts the episode number from an episode label
// e.g. "第01集" -> (1, true), "10" -> (10, true), "第12集" -> (12, true), "预告" -> (0, false)
func ExtractEpisodeNumber(name string) (int, bool) {
	clean := strings.ToLower(strings.TrimSpace(name))
	if clean == "" || strings.Contains(clean, "预告") || strings.Contains(clean, "pv") || strings.Contains(clean, "op") || strings.Contains(clean, "ed") {
		return 0, false
	}

	if m := reEpPattern1.FindStringSubmatch(clean); len(m) >= 2 {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			return n, true
		}
	}

	if m := reEpPattern2.FindStringSubmatch(clean); len(m) >= 2 {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			return n, true
		}
	}

	if m := reEpPattern3.FindStringSubmatch(clean); len(m) >= 2 {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			return n, true
		}
	}

	if m := reEpAnyNumber.FindStringSubmatch(clean); len(m) >= 2 {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			return n, true
		}
	}

	return 0, false
}

// ExtractSeasonNumber extracts season index (1, 2, 3, etc.) from title
func ExtractSeasonNumber(title string) int {
	lower := strings.ToLower(title)
	if strings.Contains(lower, "第四季") || strings.Contains(lower, "第4季") || strings.Contains(lower, "season 4") || strings.Contains(lower, "s4") {
		return 4
	}
	if strings.Contains(lower, "第三季") || strings.Contains(lower, "第3季") || strings.Contains(lower, "season 3") || strings.Contains(lower, "s3") {
		return 3
	}
	if strings.Contains(lower, "第二季") || strings.Contains(lower, "第2季") || strings.Contains(lower, "season 2") || strings.Contains(lower, "s2") {
		return 2
	}
	if strings.Contains(lower, "第一季") || strings.Contains(lower, "第1季") || strings.Contains(lower, "season 1") || strings.Contains(lower, "s1") {
		return 1
	}
	return 0 // unspecified (usually season 1 or standalone)
}

// CleanTitle removes punctuation and normalizes characters for comparison
func CleanTitle(s string) string {
	s = strings.ToLower(s)
	// Replace common synonyms
	s = strings.ReplaceAll(s, "已经死了", "已死")
	s = rePunctuation.ReplaceAllString(s, "")
	return s
}

// ScoreCandidate evaluates how closely candidate matches target (0-100)
func ScoreCandidate(target, candidate string) int {
	targetClean := CleanTitle(target)
	candClean := CleanTitle(candidate)

	if targetClean == "" || candClean == "" {
		return 0
	}

	// Exact normalized match
	if targetClean == candClean {
		return 100
	}

	targetSeason := ExtractSeasonNumber(target)
	candSeason := ExtractSeasonNumber(candidate)

	// Season mismatch check (Crucial for preventing 选A播放B across seasons!)
	if targetSeason > 0 && candSeason > 0 && targetSeason != candSeason {
		return 0 // Season mismatch! (e.g. wanted Season 3, candidate is Season 2)
	}
	if targetSeason > 1 && candSeason == 0 {
		// Target explicitly wants Season 2+ but candidate has no season indicator (likely S1)
		return 15 // Very heavy penalty
	}
	if targetSeason <= 1 && candSeason > 1 {
		// Target is Season 1 (or standalone), but candidate is Season 2+
		return 15 // Very heavy penalty
	}

	// Specific discriminator keywords (e.g. MyGO, Ave Mujica, OVA, 剧场版)
	discriminators := []string{"mygo", "avemujica", "剧场版", "ova", "外传", "特别篇"}
	for _, d := range discriminators {
		tHas := strings.Contains(targetClean, d)
		cHas := strings.Contains(candClean, d)
		if tHas && !cHas {
			return 10 // Target has specific spinoff/movie, candidate doesn't
		}
		if !tHas && cHas {
			return 10 // Target does not have spinoff/movie, but candidate does
		}
	}

	// Calculate character overlap ratio
	overlap := commonSubsequenceRatio(targetClean, candClean)
	score := int(overlap * 80)

	// Season bonus
	if targetSeason > 0 && candSeason == targetSeason {
		score += 20
	}

	if strings.Contains(candClean, targetClean) || strings.Contains(targetClean, candClean) {
		score += 15
	}

	if score > 100 {
		score = 100
	}
	return score
}

func commonSubsequenceRatio(s1, s2 string) float64 {
	r1 := []rune(s1)
	r2 := []rune(s2)
	l1 := len(r1)
	l2 := len(r2)
	if l1 == 0 || l2 == 0 {
		return 0.0
	}

	// Levenshtein distance
	matrix := make([][]int, l1+1)
	for i := range matrix {
		matrix[i] = make([]int, l2+1)
		matrix[i][0] = i
	}
	for j := 0; j <= l2; j++ {
		matrix[0][j] = j
	}

	for i := 1; i <= l1; i++ {
		for j := 1; j <= l2; j++ {
			cost := 1
			if r1[i-1] == r2[j-1] {
				cost = 0
			}
			matrix[i][j] = min(matrix[i-1][j]+1, min(matrix[i][j-1]+1, matrix[i-1][j-1]+cost))
		}
	}

	dist := matrix[l1][l2]
	maxLen := max(l1, l2)
	return 1.0 - float64(dist)/float64(maxLen)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// GenerateSearchQueries generates all reasonable search variants for a target title
func GenerateSearchQueries(title, originalTitle, cnTitle string) []string {
	seen := make(map[string]bool)
	var list []string

	add := func(q string) {
		q = strings.TrimSpace(q)
		if q != "" && !seen[q] && len([]rune(q)) >= 1 {
			seen[q] = true
			list = append(list, q)
		}
	}

	// 1. Raw title
	add(title)

	// 2. Original & CN title from Bangumi
	add(cnTitle)
	add(originalTitle)

	// 3. Punctuation stripped
	cleaned := rePunctuation.ReplaceAllString(title, " ")
	add(cleaned)
	noPunct := rePunctuation.ReplaceAllString(title, "")
	add(noPunct)

	// 4. Spaced title (e.g. "FX战士久留美" -> "FX 战士久留美")
	spaced := reAlphaHan.ReplaceAllString(title, "$1 $2")
	add(spaced)

	// 5. Chinese synonyms (e.g. "侦探已经死了" -> "侦探已死")
	if strings.Contains(title, "已经死了") {
		add(strings.ReplaceAll(title, "已经死了", "已死"))
		add(strings.ReplaceAll(cleaned, "已经死了", "已死"))
		add(strings.ReplaceAll(noPunct, "已经死了", "已死"))
	}

	// 6. Base title without season suffix
	// e.g. "药屋少女的呢喃 第三季" -> "药屋少女的呢喃"
	reSeasonSuffix := regexp.MustCompile(`(?i)(?:第[0-9一二三四五]季|season\s*[0-9]+|s[0-9]+)$`)
	base := strings.TrimSpace(reSeasonSuffix.ReplaceAllString(strings.TrimSpace(cleaned), ""))
	if base != "" && base != cleaned {
		add(base)
		add(rePunctuation.ReplaceAllString(base, ""))
	}

	// 7. Subtitle / Spinoff keywords for long titles
	// e.g. "BanG Dream! It's MyGO!!!!!" -> "MyGO"
	if strings.Contains(strings.ToLower(title), "mygo") {
		add("MyGO")
		add("BanG Dream")
	}
	if strings.Contains(strings.ToLower(title), "ave mujica") {
		add("Ave Mujica")
	}

	// 8. If original title has season, extract base of original title
	if originalTitle != "" {
		origBase := strings.TrimSpace(reSeasonSuffix.ReplaceAllString(strings.TrimSpace(originalTitle), ""))
		if origBase != "" {
			add(origBase)
		}
	}

	return list
}
