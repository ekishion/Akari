package bilibili

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
)

type DASHMuxer struct {
	ffmpegPath string
	available  bool
	checkOnce  sync.Once
}

func NewDASHMuxer() *DASHMuxer {
	m := &DASHMuxer{
		ffmpegPath: "ffmpeg",
	}
	m.checkAvailable()
	return m
}

func (m *DASHMuxer) checkAvailable() {
	m.checkOnce.Do(func() {
		path, err := exec.LookPath("ffmpeg")
		if err == nil && path != "" {
			m.ffmpegPath = path
			m.available = true
			log.Printf("[DASHMuxer] ffmpeg detected at: %s", path)
		} else {
			m.available = false
			log.Printf("[DASHMuxer] ffmpeg not found in PATH. DASH live muxing will be disabled.")
		}
	})
}

// IsAvailable returns whether ffmpeg is installed and available
func (m *DASHMuxer) IsAvailable() bool {
	return m.available
}

type flushWriter struct {
	w       io.Writer
	flusher http.Flusher
}

func (fw *flushWriter) Write(p []byte) (n int, err error) {
	n, err = fw.w.Write(p)
	if fw.flusher != nil {
		fw.flusher.Flush()
	}
	return n, err
}

// MuxStream starts an ffmpeg process to transmux video and audio DASH m4s streams into a single live MP4 stream
func (m *DASHMuxer) MuxStream(
	ctx context.Context,
	w io.Writer,
	videoURL, audioURL string,
	referer, userAgent, cookie string,
	startSeconds float64,
) error {
	if !m.available {
		return fmt.Errorf("ffmpeg is not available on this system")
	}

	if referer == "" {
		referer = defaultReferer
	}
	if userAgent == "" {
		userAgent = defaultUserAgent
	}

	args := []string{
		"-hide_banner",
		"-loglevel", "warning",
		"-fflags", "+genpts+nobuffer+discardcorrupt",
		"-threads", "2",
	}

	// Video input options
	args = append(args,
		"-user_agent", userAgent,
		"-referer", referer,
		"-timeout", "8000000",
		"-rw_timeout", "8000000",
		"-reconnect", "1",
		"-reconnect_at_eof", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "5",
	)
	if cookie != "" {
		args = append(args, "-headers", fmt.Sprintf("Cookie: %s\r\n", cookie))
	}
	if startSeconds > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", startSeconds))
	}
	args = append(args, "-i", videoURL)

	// Audio input options (if available)
	if audioURL != "" {
		args = append(args,
			"-user_agent", userAgent,
			"-referer", referer,
			"-timeout", "8000000",
			"-rw_timeout", "8000000",
			"-reconnect", "1",
			"-reconnect_at_eof", "1",
			"-reconnect_streamed", "1",
			"-reconnect_delay_max", "5",
		)
		if cookie != "" {
			args = append(args, "-headers", fmt.Sprintf("Cookie: %s\r\n", cookie))
		}
		if startSeconds > 0 {
			args = append(args, "-ss", fmt.Sprintf("%.3f", startSeconds))
		}
		args = append(args, "-i", audioURL)
	}

	// Stream mapping & fragmented MP4 container
	if audioURL != "" {
		args = append(args,
			"-map", "0:v:0",
			"-map", "1:a:0?",
			"-c:v", "copy",
			"-c:a", "copy",
			"-avoid_negative_ts", "make_zero",
			"-movflags", "frag_keyframe+empty_moov+default_base_moof+negative_cts_offsets",
			"-flush_packets", "1",
			"-f", "mp4",
			"pipe:1",
		)
	} else {
		args = append(args,
			"-map", "0:v:0",
			"-c:v", "copy",
			"-avoid_negative_ts", "make_zero",
			"-movflags", "frag_keyframe+empty_moov+default_base_moof+negative_cts_offsets",
			"-flush_packets", "1",
			"-f", "mp4",
			"pipe:1",
		)
	}

	cmd := exec.CommandContext(ctx, m.ffmpegPath, args...)
	cmd.Env = append(os.Environ(), "NO_PROXY=*.bilivideo.com,*.biliapi.net,*.bilibili.com,localhost,127.0.0.1")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to open ffmpeg stdout pipe: %w", err)
	}

	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	log.Printf("[DASHMuxer] Started live ffmpeg transmuxing (pid=%d, start=%.2fs)", cmd.Process.Pid, startSeconds)

	go func() {
		if stderr != nil {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.Contains(line, "error") || strings.Contains(line, "Error") || strings.Contains(line, "warning") || strings.Contains(line, "Warning") || strings.Contains(line, "Invalid") || strings.Contains(line, "fail") {
					log.Printf("[DASHMuxer] ffmpeg: %s", line)
				}
			}
		}
	}()

	var outWriter io.Writer = w
	if flusher, ok := w.(http.Flusher); ok {
		outWriter = &flushWriter{w: w, flusher: flusher}
	}

	// Stream stdout directly to writer
	buf := make([]byte, 32*1024)
	_, copyErr := io.CopyBuffer(outWriter, stdout, buf)

	// Ensure process terminates
	_ = cmd.Wait()

	if ctx.Err() != nil {
		log.Printf("[DASHMuxer] Client disconnected / context cancelled, killed ffmpeg (pid=%d)", cmd.Process.Pid)
		return nil
	}

	if copyErr != nil && copyErr != io.EOF {
		log.Printf("[DASHMuxer] Stream copy finished with: %v", copyErr)
	}

	return nil
}
