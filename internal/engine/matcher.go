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
func CleanTitle(s string, synonyms ...map[string]string) string {
	s = strings.ToLower(s)
	for _, synMap := range synonyms {
		for k, v := range synMap {
			if k != "" && v != "" {
				s = strings.ReplaceAll(s, strings.ToLower(k), strings.ToLower(v))
			}
		}
	}
	s = rePunctuation.ReplaceAllString(s, "")
	return s
}

func stripFormatWords(s string) string {
	s = strings.ReplaceAll(s, "大电影", "")
	s = strings.ReplaceAll(s, "动画电影", "")
	s = strings.ReplaceAll(s, "电影版", "")
	s = strings.ReplaceAll(s, "剧场版", "")
	s = strings.ReplaceAll(s, "电影", "")
	s = strings.ReplaceAll(s, "特别篇", "")
	s = strings.ReplaceAll(s, "总集篇", "")
	s = strings.ReplaceAll(s, "ova", "")
	s = strings.ReplaceAll(s, "oad", "")
	s = strings.ReplaceAll(s, "sp", "")
	return strings.TrimSpace(s)
}

func hasFormatWords(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "剧场版") ||
		strings.Contains(lower, "电影版") ||
		strings.Contains(lower, "大电影") ||
		strings.Contains(lower, "电影") ||
		strings.Contains(lower, "动画电影") ||
		strings.Contains(lower, "movie")
}

// ScoreCandidate evaluates how closely candidate matches target (0-100) with movie/TV awareness
func ScoreCandidate(target, candidate string, isMovie bool, synonyms ...map[string]string) int {
	var syn map[string]string
	if len(synonyms) > 0 {
		syn = synonyms[0]
	}
	targetClean := CleanTitle(target, syn)
	candClean := CleanTitle(candidate, syn)

	if targetClean == "" || candClean == "" {
		return 0
	}

	candHasFormat := hasFormatWords(candidate) || hasFormatWords(candClean)
	targetHasFormat := hasFormatWords(target) || hasFormatWords(targetClean)

	targetBase := stripFormatWords(targetClean)
	candBase := stripFormatWords(candClean)

	// Target is TV series, but candidate is a movie/theatrical edition
	if !isMovie && !targetHasFormat && candHasFormat {
		return 20 // Heavy penalty to prevent playing movie when watching TV series
	}

	// Exact base match handling (e.g. "罗小黑战记" vs "罗小黑战记 电影版")
	if targetBase != "" && candBase != "" && targetBase == candBase {
		if isMovie {
			if candHasFormat {
				return 100 // Exact movie target matching explicit movie candidate
			}
			return 70 // Exact base match but lacks movie marker, ranks below explicit movie
		}
		if !candHasFormat {
			return 100 // Exact TV series match
		}
	}

	// Exact normalized string match
	if targetClean == candClean {
		if isMovie && !candHasFormat {
			return 75 // Plain name without movie marker when target is movie
		}
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

	rTarget := []rune(targetClean)
	rCand := []rune(candClean)
	lenMin := min(len(rTarget), len(rCand))
	lenMax := max(len(rTarget), len(rCand))
	lenRatio := float64(lenMin) / float64(lenMax)

	// Check common prefix for distinct franchise sub-brand / subtitle divergence
	commonPrefixLen := 0
	for commonPrefixLen < lenMin && rTarget[commonPrefixLen] == rCand[commonPrefixLen] {
		commonPrefixLen++
	}

	targetRemainder := len(rTarget) - commonPrefixLen
	candRemainder := len(rCand) - commonPrefixLen

	// Subtitle divergence: shared franchise prefix but diverging into different sub-titles
	// e.g. "bangdream" + "itsmygo" (7 chars) vs "bangdream" + "avemujica" (9 chars)
	if commonPrefixLen >= 4 && targetRemainder >= 3 && candRemainder >= 3 {
		return 15 // Distinct divergent spinoffs/subtitles
	}

	// Missing major subtitle: one is a prefix of the other, but missing a substantial subtitle
	// e.g. "bangdreamitsmygo" vs "bangdream" (target has 7 extra chars, lenRatio < 0.70)
	if (strings.Contains(candClean, targetClean) || strings.Contains(targetClean, candClean)) && lenRatio < 0.70 && (lenMax-lenMin) >= 4 {
		if targetBase != candBase {
			return 20 // Missing major subtitle / parent franchise mismatch
		}
	}

	// Calculate character overlap ratio
	overlap := commonSubsequenceRatio(targetClean, candClean)
	score := int(overlap * 80)

	// Also check overlap of base titles without format noise
	if targetBase != "" && candBase != "" {
		baseOverlap := commonSubsequenceRatio(targetBase, candBase)
		baseScore := int(baseOverlap * 85)
		if baseScore > score {
			score = baseScore
		}
	}

	// Season bonus
	if targetSeason > 0 && candSeason == targetSeason {
		score += 20
	}

	if isMovie && candHasFormat {
		score += 15
	}

	if lenRatio >= 0.75 {
		if strings.Contains(candClean, targetClean) || strings.Contains(targetClean, candClean) ||
			(targetBase != "" && (strings.Contains(candBase, targetBase) || strings.Contains(targetBase, candBase))) {
			score += 15
		}
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

// GenerateSearchQueries generates all reasonable search variants for a target title dynamically
func GenerateSearchQueries(title, originalTitle, cnTitle string, isMovie bool, synonyms map[string]string, extraAliases ...string) []string {
	seen := make(map[string]bool)
	var list []string

	add := func(q string) {
		q = strings.TrimSpace(q)
		if q != "" && !seen[q] && len([]rune(q)) >= 1 {
			seen[q] = true
			list = append(list, q)
		}
	}

	// If target is a Movie / 剧场版, prioritize movie query variants first
	if isMovie {
		for _, baseT := range []string{title, cnTitle} {
			if baseT != "" {
				cleanB := rePunctuation.ReplaceAllString(baseT, "")
				if !hasFormatWords(cleanB) {
					add(cleanB + " 电影版")
					add(cleanB + " 剧场版")
					add(cleanB + "电影版")
					add(cleanB + "剧场版")
					add(cleanB + "大电影")
					add(cleanB + " 电影")
				}
			}
		}
	}

	// 1. Raw title, CN title, and original title
	add(title)
	add(cnTitle)
	add(originalTitle)

	// 2. Punctuation stripped
	cleaned := rePunctuation.ReplaceAllString(title, " ")
	add(cleaned)
	noPunct := rePunctuation.ReplaceAllString(title, "")
	add(noPunct)

	// Also strip punctuation from originalTitle and cnTitle
	if originalTitle != "" {
		origCleaned := rePunctuation.ReplaceAllString(originalTitle, " ")
		add(origCleaned)
		origNoPunct := rePunctuation.ReplaceAllString(originalTitle, "")
		add(origNoPunct)
	}
	if cnTitle != "" {
		cnCleaned := rePunctuation.ReplaceAllString(cnTitle, " ")
		add(cnCleaned)
		cnNoPunct := rePunctuation.ReplaceAllString(cnTitle, "")
		add(cnNoPunct)
	}

	// 3. Spaced alphanumeric title (e.g. "FX战士久留美" -> "FX 战士久留美")
	spaced := reAlphaHan.ReplaceAllString(title, "$1 $2")
	add(spaced)

	// 4. Base title without format words
	baseNoFormat := strings.TrimSpace(stripFormatWords(noPunct))
	if baseNoFormat != "" && baseNoFormat != noPunct {
		add(baseNoFormat)
	}

	// 5. Base title without season suffix
	reSeasonSuffix := regexp.MustCompile(`(?i)(?:第[0-9一二三四五]季|season\s*[0-9]+|s[0-9]+)$`)
	base := strings.TrimSpace(reSeasonSuffix.ReplaceAllString(strings.TrimSpace(cleaned), ""))
	if base != "" && base != cleaned {
		add(base)
		add(rePunctuation.ReplaceAllString(base, ""))
	}

	// 6. Dynamic Synonym Applications (bidirectional matching and prefix swapping)
	currentSnap := make([]string, len(list))
	copy(currentSnap, list)

	for _, item := range currentSnap {
		for pat, rep := range synonyms {
			if pat == "" || rep == "" {
				continue
			}
			// Replace pattern with replacement
			if strings.Contains(item, pat) {
				subbed := strings.ReplaceAll(item, pat, rep)
				add(subbed)
				add(rePunctuation.ReplaceAllString(subbed, ""))
			}
			// Replace replacement with pattern (bidirectional)
			if strings.Contains(item, rep) {
				subbed := strings.ReplaceAll(item, rep, pat)
				add(subbed)
				add(rePunctuation.ReplaceAllString(subbed, ""))
			}
			// Prefix swapping
			if strings.HasPrefix(item, pat) {
				trimmed := strings.TrimPrefix(item, pat)
				add(rep + trimmed)
				if len([]rune(trimmed)) >= 2 {
					add(trimmed)
				}
			} else if strings.HasPrefix(item, rep) {
				trimmed := strings.TrimPrefix(item, rep)
				add(pat + trimmed)
				if len([]rune(trimmed)) >= 2 {
					add(trimmed)
				}
			}
		}
	}

	// 7. Extra Aliases (from Bangumi Infobox/Tags and DB Subject Aliases)
	for _, alias := range extraAliases {
		add(alias)
		aliasNoP := rePunctuation.ReplaceAllString(alias, "")
		add(aliasNoP)

		// Apply dynamic synonyms to extra aliases
		for pat, rep := range synonyms {
			if pat == "" || rep == "" {
				continue
			}
			if strings.Contains(aliasNoP, pat) {
				add(strings.ReplaceAll(aliasNoP, pat, rep))
			}
			if strings.Contains(aliasNoP, rep) {
				add(strings.ReplaceAll(aliasNoP, rep, pat))
			}
			if strings.HasPrefix(aliasNoP, pat) {
				trimmed := strings.TrimPrefix(aliasNoP, pat)
				add(rep + trimmed)
				if len([]rune(trimmed)) >= 2 {
					add(trimmed)
				}
			} else if strings.HasPrefix(aliasNoP, rep) {
				trimmed := strings.TrimPrefix(aliasNoP, rep)
				add(pat + trimmed)
				if len([]rune(trimmed)) >= 2 {
					add(trimmed)
				}
			}
		}
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
