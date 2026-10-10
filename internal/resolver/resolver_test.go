package resolver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStreamResolver_ExtractMacCMS(t *testing.T) {
	r := NewStreamResolver()

	// MacCMS standard plain URL
	html1 := `<script type="text/javascript">var player_aaaa={"flag":"play","encrypt":0,"url":"https:\/\/play.xfvod.pro:8088\/video\/01.mp4","from":"xfy2"};</script>`
	url1, ok1 := r.extractMacCMS(html1, "https://dm1.xfdm.pro/")
	if !ok1 || url1 != "https://play.xfvod.pro:8088/video/01.mp4" {
		t.Fatalf("expected extracted url, got %v (%s)", ok1, url1)
	}

	// MacCMS with let/const and script tag closing
	html2 := `<script>let player_aaaa={"encrypt":0,"url":"https://cdn.example.com/live/index.m3u8"};</script>`
	url2, ok2 := r.extractMacCMS(html2, "https://example.com/")
	if !ok2 || url2 != "https://cdn.example.com/live/index.m3u8" {
		t.Fatalf("expected extracted url, got %v (%s)", ok2, url2)
	}
}

func TestStreamResolver_DecodeVideoSourceParam(t *testing.T) {
	raw := "https://jx.player.com/?url=https%3A%2F%2Fcdn.example.com%2Fvideo.m3u8&autoplay=true"
	extracted := decodeVideoSourceParam(raw)
	if extracted != "https://cdn.example.com/video.m3u8" {
		t.Fatalf("expected decoded url, got %s", extracted)
	}
}

func TestStreamResolver_ExtractScriptConfigs(t *testing.T) {
	r := NewStreamResolver()

	html := `
	var art = new Artplayer({
		container: '.MacPlayer',
		url: 'https://cdn.yzzy.com/2026/index.m3u8',
		type: 'm3u8'
	});
	`
	url, ok := r.extractScriptConfigs(html)
	if !ok || url != "https://cdn.yzzy.com/2026/index.m3u8" {
		t.Fatalf("expected artplayer url, got %v (%s)", ok, url)
	}
}

func TestStreamResolver_DirectMediaURL(t *testing.T) {
	r := NewStreamResolver()
	ctx := context.Background()

	stream, err := r.Resolve(ctx, "https://play.example.com/anime.mp4", "", "")
	if err != nil || stream == nil || stream.RealURL != "https://play.example.com/anime.mp4" || stream.Format != "mp4" {
		t.Fatalf("expected direct mp4, got %v, err: %v", stream, err)
	}
}

func TestStreamResolver_ResolveRealXfdm(t *testing.T) {
	r := NewStreamResolver()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := r.Resolve(ctx, "https://dm1.xfdm.pro/watch/3517/1/1.html", "https://dm1.xfdm.pro/", "Mozilla/5.0")
	if err != nil {
		t.Logf("Resolve encountered error (network dependent): %v", err)
		return
	}
	t.Logf("Resolved real stream URL: %s (format: %s)", stream.RealURL, stream.Format)
	if stream.RealURL == "" {
		t.Fatalf("expected non-empty real stream url")
	}
}

func TestStreamResolver_ResolveEzdmw(t *testing.T) {
	r := NewStreamResolver()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := r.Resolve(ctx, "https://m.ezdmw.org/Index/video/102631.html", "https://m.ezdmw.org/", "Mozilla/5.0")
	if err != nil {
		t.Logf("Resolve ezdmw error: %v", err)
		return
	}
	t.Logf("Resolved ezdmw stream: %+v", stream)
}

func TestStreamResolver_VerifyPlayable(t *testing.T) {
	r := NewStreamResolver()
	ctx := context.Background()

	// Mock upstream server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/video.mp4":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isomfake-video-payload"))
		case "/dead.mp4":
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Bad Request\n"))
		case "/d/blocked.mp4":
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Bad Request\n"))
		case "/p/blocked.mp4":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("\x00\x00\x00\x18ftypisom\x00\x00\x00\x00isommp42"))
		case "/playlist.m3u8":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("#EXTM3U\n#EXT-X-VERSION:3\n#EXTINF:10.0,\nseg1.ts\n"))
		case "/fake.m3u8":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html>Error Page</html>"))
		case "/fake.webp":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("RIFF\x20\x00\x00\x00WEBPVP8 ...fake-image-bytes..."))
		case "/fake.png":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR..."))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	// 1. Playable MP4
	streamOK := &ResolvedStream{
		RealURL: ts.URL + "/video.mp4",
		Format:  "mp4",
	}
	u, ok := r.VerifyPlayable(ctx, streamOK)
	if !ok || u != ts.URL+"/video.mp4" {
		t.Fatalf("expected playable mp4, got %v, %s", ok, u)
	}

	// 2. Dead MP4 (400 Bad Request)
	streamDead := &ResolvedStream{
		RealURL: ts.URL + "/dead.mp4",
		Format:  "mp4",
	}
	_, okDead := r.VerifyPlayable(ctx, streamDead)
	if okDead {
		t.Fatalf("expected dead mp4 to fail verification")
	}

	// 3. AList /d/ blocked -> /p/ fallback works
	streamAList := &ResolvedStream{
		RealURL: ts.URL + "/d/blocked.mp4",
		Format:  "mp4",
	}
	uAList, okAList := r.VerifyPlayable(ctx, streamAList)
	if !okAList || uAList != ts.URL+"/p/blocked.mp4" {
		t.Fatalf("expected AList /p/ fallback to succeed, got %v, %s", okAList, uAList)
	}

	// 4. Valid M3U8
	streamM3U8 := &ResolvedStream{
		RealURL: ts.URL + "/playlist.m3u8",
		Format:  "m3u8",
	}
	uM3U8, okM3U8 := r.VerifyPlayable(ctx, streamM3U8)
	if !okM3U8 || uM3U8 != ts.URL+"/playlist.m3u8" {
		t.Fatalf("expected valid m3u8 to succeed, got %v, %s", okM3U8, uM3U8)
	}

	// 5. HTML error masquerading as M3U8
	streamFakeM3U8 := &ResolvedStream{
		RealURL: ts.URL + "/fake.m3u8",
		Format:  "m3u8",
	}
	_, okFake := r.VerifyPlayable(ctx, streamFakeM3U8)
	if okFake {
		t.Fatalf("expected fake m3u8 without #EXTM3U to fail verification")
	}

	// 6. Fake WebP masquerading as video
	streamFakeWebP := &ResolvedStream{
		RealURL: ts.URL + "/fake.webp",
		Format:  "mp4",
	}
	_, okFakeWebP := r.VerifyPlayable(ctx, streamFakeWebP)
	if okFakeWebP {
		t.Fatalf("expected fake webp to fail container verification")
	}

	// 7. Fake PNG masquerading as video
	streamFakePNG := &ResolvedStream{
		RealURL: ts.URL + "/fake.png",
		Format:  "mp4",
	}
	_, okFakePNG := r.VerifyPlayable(ctx, streamFakePNG)
	if okFakePNG {
		t.Fatalf("expected fake png to fail container verification")
	}
}
