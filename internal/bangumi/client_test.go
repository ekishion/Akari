package bangumi

import (
	"sync"
	"testing"
	"time"
)

func TestConcurrentBangumi(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live network test in short mode")
	}
	client := NewClient("")
	start := time.Now()
	var wg sync.WaitGroup

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			var err error
			var count int
			if idx == 0 {
				res, e := client.GetCalendar()
				err = e
				count = len(res)
			} else {
				res, e := client.GetTrending(20, 0)
				err = e
				count = len(res)
			}
			t.Logf("[%d] Done in %v, err=%v, count=%d", idx, time.Since(start), err, count)
		}(i)
	}
	wg.Wait()
	t.Logf("Total time: %v", time.Since(start))
}

func TestBaseUrlAndMirror(t *testing.T) {
	client := NewClient("https://next.bgm.tv/")
	if client.GetBaseUrl() != "https://next.bgm.tv" {
		t.Fatalf("expected https://next.bgm.tv, got %s", client.GetBaseUrl())
	}

	client.SetBaseUrl("https://api.bgm.tv/")
	if client.GetBaseUrl() != "https://api.bgm.tv" {
		t.Fatalf("expected https://api.bgm.tv, got %s", client.GetBaseUrl())
	}

	client.SetBaseUrl("")
	if client.GetBaseUrl() != DefaultAPIBase {
		t.Fatalf("expected default %s, got %s", DefaultAPIBase, client.GetBaseUrl())
	}
}

func TestRewriteImageUrl(t *testing.T) {
	client := NewClient("")
	orig := "http://lain.bgm.tv/pic/cover/l/1c/b4/390200_1IA55.jpg"
	rewritten := client.RewriteImageUrl(orig)
	expected := "https://lain.bgm.tv/pic/cover/l/1c/b4/390200_1IA55.jpg"
	if rewritten != expected {
		t.Fatalf("expected %s, got %s", expected, rewritten)
	}

	origHttps := "https://lain.bgm.tv/pic/cover/l/1c/b4/390200_1IA55.jpg"
	rewrittenHttps := client.RewriteImageUrl(origHttps)
	if rewrittenHttps != expected {
		t.Fatalf("expected %s, got %s", expected, rewrittenHttps)
	}

	client.SetImageHost("https://custom-image-mirror.com")
	customRewritten := client.RewriteImageUrl(orig)
	if customRewritten != "https://custom-image-mirror.com/pic/cover/l/1c/b4/390200_1IA55.jpg" {
		t.Fatalf("expected custom host after SetImageHost, got %s", customRewritten)
	}
}

func TestInvalidEndpoint(t *testing.T) {
	client := NewClient("")
	_, err := client.TestEndpoint("invalid-schema://bad")
	if err == nil {
		t.Fatalf("expected error for invalid url schema")
	}
}

