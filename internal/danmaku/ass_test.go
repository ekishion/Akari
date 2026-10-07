package danmaku

import (
	"strings"
	"testing"
)

func TestCommentsToAss(t *testing.T) {
	comments := []DanmakuComment{
		{
			Time:     1.5,
			Type:     1, // 滚动
			ColorInt: 16711680, // 0xFF0000 红色
			Text:     "第一条滚动弹幕！",
		},
		{
			Time:     1.8,
			Type:     1, // 紧接着的一条滚动弹幕（应触发轨道分配）
			ColorInt: 65280, // 0x00FF00 绿色
			Text:     "紧接着的第二条弹幕",
		},
		{
			Time:     2.0,
			Type:     5, // 顶部固定
			ColorInt: 16777215, // 白色
			Text:     "前方高能警告！",
		},
		{
			Time:     3.0,
			Type:     4, // 底部固定
			ColorInt: 255, // 0x0000FF 蓝色
			Text:     "名场面打卡",
		},
	}

	ass := CommentsToAss(comments, "测试番剧")

	// 1. Check Header
	if !strings.Contains(ass, "[Script Info]") {
		t.Errorf("Missing [Script Info]")
	}
	if !strings.Contains(ass, "PlayResX: 1920") {
		t.Errorf("Missing PlayResX")
	}

	// 2. Check Styles
	if !strings.Contains(ass, "Style: DanmakuScroll") {
		t.Errorf("Missing DanmakuScroll style")
	}
	if !strings.Contains(ass, "Style: DanmakuTop") {
		t.Errorf("Missing DanmakuTop style")
	}
	if !strings.Contains(ass, "Style: DanmakuBottom") {
		t.Errorf("Missing DanmakuBottom style")
	}

	// 3. Check Scroll Move tag
	if !strings.Contains(ass, "\\move(1920,") {
		t.Errorf("Missing \\move tag for scroll danmaku")
	}

	// 4. Check Top and Bottom Pos tags
	if !strings.Contains(ass, "\\pos(960,") {
		t.Errorf("Missing \\pos tag for fixed danmaku")
	}

	// 5. Check Colors (ASS expects BGR format)
	// 红色 0xFF0000 -> BGR &H000000FF&
	if !strings.Contains(ass, "0000FF&") {
		t.Errorf("Red color BGR conversion mismatch")
	}
	// 绿色 0x00FF00 -> BGR &H0000FF00&
	if !strings.Contains(ass, "00FF00&") {
		t.Errorf("Green color BGR conversion mismatch")
	}
	// 蓝色 0x0000FF -> BGR &H00FF0000&
	if !strings.Contains(ass, "FF0000&") {
		t.Errorf("Blue color BGR conversion mismatch")
	}

	t.Logf("Generated ASS output:\n%s", ass)
}
