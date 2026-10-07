package danmaku

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	AssCanvasWidth  = 1920
	AssCanvasHeight = 1080
	ScrollDuration  = 8.0 // 滚动弹幕在屏幕上运行的时间（秒）
	FixedDuration   = 4.0 // 顶部/底部固定弹幕显示时间（秒）
	FontSize        = 48
	LineHeight      = 58
	TotalTracks     = 14
)

// CommentsToAss converts a slice of DanmakuComment into complete ASS subtitle text
func CommentsToAss(comments []DanmakuComment, title string) string {
	if len(comments) == 0 {
		return GenerateEmptyAss(title)
	}

	// 1. Sort comments by timeline
	sort.SliceStable(comments, func(i, j int) bool {
		return comments[i].Time < comments[j].Time
	})

	var sb strings.Builder

	// Header
	sb.WriteString("[Script Info]\n")
	sb.WriteString(fmt.Sprintf("Title: %s - Kazumi Danmaku\n", title))
	sb.WriteString("ScriptType: v4.00+\n")
	sb.WriteString(fmt.Sprintf("PlayResX: %d\n", AssCanvasWidth))
	sb.WriteString(fmt.Sprintf("PlayResY: %d\n", AssCanvasHeight))
	sb.WriteString("ScaledBorderAndShadow: yes\n")
	sb.WriteString("WrapStyle: 2\n\n")

	// Styles
	sb.WriteString("[V4+ Styles]\n")
	sb.WriteString("Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
	// Default scroll style (Alignment=7: Top-Left)
	sb.WriteString("Style: DanmakuScroll,MiSans,48,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,0,7,0,0,0,1\n")
	// Top fixed style (Alignment=8: Top-Center)
	sb.WriteString("Style: DanmakuTop,MiSans,48,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,0,8,0,0,0,1\n")
	// Bottom fixed style (Alignment=2: Bottom-Center)
	sb.WriteString("Style: DanmakuBottom,MiSans,48,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,0,2,0,0,0,1\n\n")

	// Events
	sb.WriteString("[Events]\n")
	sb.WriteString("Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")

	// State for multi-track collision detection
	trackTailLeaveTime := make([]float64, TotalTracks) // 各跑道上一条弹幕尾部完全进入屏幕的时间
	topTrackUntil := make([]float64, 6)
	bottomTrackUntil := make([]float64, 6)

	for _, c := range comments {
		if c.Time < 0 {
			continue
		}
		text := sanitizeAssText(c.Text)
		if text == "" {
			continue
		}

		colorTag := formatAssColor(c.ColorInt)
		startTime := c.Time

		switch c.Type {
		case 5: // 顶部固定
			endTime := startTime + FixedDuration
			track := allocateFixedTrack(topTrackUntil, startTime)
			y := 60 + track*LineHeight
			sb.WriteString(fmt.Sprintf("Dialogue: 2,%s,%s,DanmakuTop,,0,0,0,,{\\pos(960,%d)%s}%s\n",
				FormatAssTime(startTime), FormatAssTime(endTime), y, colorTag, text))

		case 4: // 底部固定
			endTime := startTime + FixedDuration
			track := allocateFixedTrack(bottomTrackUntil, startTime)
			y := AssCanvasHeight - 60 - track*LineHeight
			sb.WriteString(fmt.Sprintf("Dialogue: 2,%s,%s,DanmakuBottom,,0,0,0,,{\\pos(960,%d)%s}%s\n",
				FormatAssTime(startTime), FormatAssTime(endTime), y, colorTag, text))

		default: // 1: 滚动弹幕
			endTime := startTime + ScrollDuration
			textWidth := estimateTextWidth(text)
			totalDistance := float64(AssCanvasWidth + textWidth)
			speed := totalDistance / ScrollDuration

			// 计算弹幕尾部进入屏幕右边缘的时间
			tailEnterScreenTime := startTime + float64(textWidth)/speed

			track := allocateScrollTrack(trackTailLeaveTime, startTime)
			trackTailLeaveTime[track] = tailEnterScreenTime

			y := 50 + track*LineHeight
			endX := -textWidth

			sb.WriteString(fmt.Sprintf("Dialogue: 1,%s,%s,DanmakuScroll,,0,0,0,,{\\move(%d,%d,%d,%d)%s}%s\n",
				FormatAssTime(startTime), FormatAssTime(endTime),
				AssCanvasWidth, y, endX, y, colorTag, text))
		}
	}

	return sb.String()
}

func allocateScrollTrack(tracks []float64, currentTime float64) int {
	bestTrack := 0
	earliestTime := math.MaxFloat64

	for i, t := range tracks {
		if currentTime >= t {
			return i
		}
		if t < earliestTime {
			earliestTime = t
			bestTrack = i
		}
	}
	return bestTrack
}

func allocateFixedTrack(tracks []float64, currentTime float64) int {
	for i, t := range tracks {
		if currentTime >= t {
			tracks[i] = currentTime + FixedDuration
			return i
		}
	}
	// Fallback to track 0 if all busy
	tracks[0] = currentTime + FixedDuration
	return 0
}

func estimateTextWidth(text string) int {
	width := 0
	for _, r := range text {
		if r > 127 {
			width += FontSize
		} else {
			width += FontSize / 2
		}
	}
	if width < FontSize {
		width = FontSize
	}
	return width
}

func formatAssColor(colorInt int) string {
	if colorInt <= 0 || colorInt == 16777215 { // 白色默认
		return ""
	}

	r := (colorInt >> 16) & 0xFF
	g := (colorInt >> 8) & 0xFF
	b := colorInt & 0xFF

	// ASS color format is &H00BBGGRR&
	return fmt.Sprintf("{\\1c&H00%02X%02X%02X&}", b, g, r)
}

func FormatAssTime(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	h := int(seconds) / 3600
	m := (int(seconds) % 3600) / 60
	s := int(seconds) % 60
	cs := int((seconds - math.Floor(seconds)) * 100)
	return fmt.Sprintf("%d:%02d:%02d.%02d", h, m, s, cs)
}

func sanitizeAssText(text string) string {
	text = strings.ReplaceAll(text, "\r", "")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "{", "(")
	text = strings.ReplaceAll(text, "}", ")")
	if utf8.RuneCountInString(text) > 100 {
		runes := []rune(text)
		text = string(runes[:100])
	}
	return strings.TrimSpace(text)
}

func GenerateEmptyAss(title string) string {
	return fmt.Sprintf(`[Script Info]
Title: %s - Kazumi Danmaku
ScriptType: v4.00+
PlayResX: 1920
PlayResY: 1080

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Default,MiSans,48,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,0,2,20,20,20,1

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:01.00,0:00:06.00,Default,,0,0,0,,\an8\pos(960,60)Kazumi 弹幕已加载 (暂无弹幕评论)
`, title)
}
