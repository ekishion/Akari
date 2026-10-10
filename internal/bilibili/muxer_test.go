package bilibili

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestDASHMuxer_Availability(t *testing.T) {
	muxer := NewDASHMuxer()
	// Test if muxer doesn't panic on initialization
	t.Logf("DASHMuxer available: %v, path: %s", muxer.IsAvailable(), muxer.ffmpegPath)
}

func TestDASHMuxer_InvalidStream(t *testing.T) {
	muxer := NewDASHMuxer()
	if !muxer.IsAvailable() {
		t.Skip("ffmpeg not available, skipping test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var buf bytes.Buffer
	// Running with invalid URLs should terminate gracefully when context expires
	_ = muxer.MuxStream(ctx, &buf, "http://127.0.0.1:9/invalid_v.m4s", "http://127.0.0.1:9/invalid_a.m4s", "", "", "", 0)
}
