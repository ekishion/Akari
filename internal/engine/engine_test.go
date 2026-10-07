package engine

import (
	"context"
	"testing"
	"time"
)

func TestEngine_SearchAndQueryChapters(t *testing.T) {
	eng := NewEngine()

	plugin := &Plugin{
		Name:          "稀饭动漫",
		BaseURL:       "https://dm1.xfdm.pro/",
		SearchURL:     "https://dm1.xfdm.pro/search.html?wd=@keyword",
		SearchList:    "//div[contains(@class,'public-list-box')]",
		SearchName:    "//div[contains(@class,'thumb-txt')]",
		SearchResult:  "//a[contains(@class,'public-list-exp')]",
		ChapterRoads:  "//ul[contains(@class,'anthology-list-play')]",
		ChapterResult: "//li/a",
		Referer:       "https://dm1.xfdm.pro/",
		SearchMode:    "xpath",
		ChapterMode:   "xpath",
		Enabled:       true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results, err := eng.Search(ctx, plugin, "超超超超超喜欢你的100个女朋友")
	if err != nil {
		t.Logf("Search encountered error (network dependent): %v", err)
		return
	}

	if len(results) == 0 {
		t.Logf("Search returned 0 results (network dependent)")
		return
	}

	t.Logf("Found %d results, first: %s -> %s", len(results), results[0].Name, results[0].Src)

	roads, err := eng.QueryChapters(ctx, plugin, results[0].Src)
	if err != nil {
		t.Logf("QueryChapters encountered error: %v", err)
		return
	}

	t.Logf("Found %d roads", len(roads))
	if len(roads) > 0 && len(roads[0].Episodes) > 0 {
		t.Logf("Road 1 Ep 1: %s -> %s", roads[0].Episodes[0].Name, roads[0].Episodes[0].URL)
	}
}

func TestEngine_Search_EZDMW(t *testing.T) {
	eng := NewEngine()

	plugin := &Plugin{
		Name:          "极速动漫",
		BaseURL:       "https://m.ezdmw.org/",
		SearchURL:     "https://m.ezdmw.org/Index/search.html?searchText=@keyword",
		SearchList:    "//section[@id='some_drama']/div",
		SearchName:    "//p",
		SearchResult:  "//a",
		ChapterRoads:  "//section[contains(@class,'anthology')]",
		ChapterResult: "//a[contains(@class,'circuit_switch1')] | //a[contains(@class,'circuit_switch2')] | //a[contains(@class,'circuit_switch3')]",
		Referer:       "https://m.ezdmw.org/",
		SearchMode:    "xpath",
		ChapterMode:   "xpath",
		Enabled:       true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results, err := eng.Search(ctx, plugin, "久留美")
	if err != nil {
		t.Logf("Search encountered error: %v", err)
		return
	}

	t.Logf("ezdmw returned %d results", len(results))
	for _, r := range results {
		t.Logf("Result: %s -> %s", r.Name, r.Src)
	}

	if len(results) > 0 {
		roads, err := eng.QueryChapters(ctx, plugin, results[0].Src)
		if err != nil {
			t.Logf("Chapters error: %v", err)
		} else {
			t.Logf("ezdmw found %d roads", len(roads))
			for i, rd := range roads {
				t.Logf("Road %d: %s, episodes: %d", i+1, rd.Name, len(rd.Episodes))
				if len(rd.Episodes) > 0 {
					t.Logf("  Ep 1: %s -> %s", rd.Episodes[0].Name, rd.Episodes[0].URL)
				}
			}
		}
	}
}

func TestEngine_SearchMultiTarget(t *testing.T) {
	eng := NewEngine()
	xfdm := &Plugin{
		Name:          "稀饭动漫",
		BaseURL:       "https://dm1.xfdm.pro/",
		SearchURL:     "https://dm1.xfdm.pro/search.html?wd=@keyword",
		SearchList:    "//div[contains(@class,'public-list-box')]",
		SearchName:    "//div[contains(@class,'thumb-txt')]",
		SearchResult:  "//a[contains(@class,'public-list-exp')]",
		ChapterRoads:  "//ul[contains(@class,'anthology-list-play')]",
		ChapterResult: "//li/a",
		Referer:       "https://dm1.xfdm.pro/",
		SearchMode:    "xpath",
		ChapterMode:   "xpath",
		Enabled:       true,
	}
	mxdm := &Plugin{
		Name:          "MX动漫",
		BaseURL:       "https://www.dcc3.com",
		SearchURL:     "https://www.dcc3.com/search/?wd=@keyword",
		SearchList:    "//div[contains(@class, 'search')]/ul/li",
		SearchName:    "//h3/a",
		SearchResult:  "//h3/a",
		ChapterRoads:  "//div[@class='playlist']/div[@class='row']/ul",
		ChapterResult: "//li/a",
		Referer:       "https://www.dcc3.com/",
		SearchMode:    "xpath",
		ChapterMode:   "xpath",
		Enabled:       true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	queries := []string{
		"BanG Dream! It's MyGO!!!!!",
		"MyGO",
		"BanG Dream",
		"侦探已经死了。 第二季",
		"侦探已经死了",
		"侦探已死",
		"药屋少女的呢喃 第三季",
		"药屋少女的呢喃",
	}

	for _, q := range queries {
		resXf, _ := eng.Search(ctx, xfdm, q)
		t.Logf("Query [%s] on 稀饭动漫: %d results", q, len(resXf))
		for _, r := range resXf {
			t.Logf("  xfdm: %s (%s)", r.Name, r.Src)
		}

		resMx, _ := eng.Search(ctx, mxdm, q)
		t.Logf("Query [%s] on MX动漫: %d results", q, len(resMx))
		for _, r := range resMx {
			t.Logf("  mxdm: %s (%s)", r.Name, r.Src)
		}
	}
}

