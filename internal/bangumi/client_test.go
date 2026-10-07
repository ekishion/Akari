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
	client := NewClient("https://mirror.bgm.rin.cat/")
	if client.GetBaseUrl() != "https://mirror.bgm.rin.cat" {
		t.Fatalf("expected https://mirror.bgm.rin.cat, got %s", client.GetBaseUrl())
	}

	client.SetBaseUrl("https://chii.ai/")
	if client.GetBaseUrl() != "https://chii.ai" {
		t.Fatalf("expected https://chii.ai, got %s", client.GetBaseUrl())
	}

	client.SetBaseUrl("")
	if client.GetBaseUrl() != "https://api.bgm.tv" {
		t.Fatalf("expected default https://api.bgm.tv, got %s", client.GetBaseUrl())
	}
}

func TestInvalidEndpoint(t *testing.T) {
	client := NewClient("")
	_, err := client.TestEndpoint("invalid-schema://bad")
	if err == nil {
		t.Fatalf("expected error for invalid url schema")
	}
}

