package engine

import (
	"encoding/json"
	"fmt"
)

type RuleMode string

const (
	ModeXPath RuleMode = "xpath"
	ModeAPI   RuleMode = "api"
)

type ApiEpisodePageConfig struct {
	URL   string            `json:"url"`
	Query map[string]string `json:"query,omitempty"`
}

type ApiRequestConfig struct {
	URL      string            `json:"url"`
	Method   string            `json:"method"`
	Headers  map[string]string `json:"headers,omitempty"`
	BodyType string            `json:"bodyType,omitempty"`
	Body     any               `json:"body,omitempty"`
}

type ApiSearchConfig struct {
	Request       ApiRequestConfig `json:"request"`
	ItemsJsonPath string           `json:"itemsJsonPath"`
	ListPath      string           `json:"listPath"`
	NameJsonPath  string           `json:"nameJsonPath"`
	NamePath      string           `json:"namePath"`
	SrcJsonPath   string           `json:"srcJsonPath"`
	SourcePath    string           `json:"sourcePath"`
}

type ApiChapterConfig struct {
	Request          ApiRequestConfig     `json:"request"`
	RoadsJsonPath    string               `json:"roadsJsonPath"`
	RoadsPath        string               `json:"roadsPath"`
	RoadNameJsonPath string               `json:"roadNameJsonPath"`
	RoadNamePath     string               `json:"roadNamePath"`
	UrlsJsonPath     string               `json:"urlsJsonPath"`
	EpisodeUrlPath   string               `json:"episodeUrlPath"`
	NamesJsonPath    string               `json:"namesJsonPath"`
	EpisodeNamePath  string               `json:"episodeNamePath"`
	EpisodePage      ApiEpisodePageConfig `json:"episodePage,omitempty"`
}

// Plugin matches Kazumi Schema V8
type Plugin struct {
	ID               string           `json:"id,omitempty"`
	Api              string           `json:"api"`
	Type             string           `json:"type"`
	Name             string           `json:"name"`
	Version          string           `json:"version"`
	MultiSources     bool             `json:"muliSources"`
	UseWebview       bool             `json:"useWebview"`
	UseNativePlayer  bool             `json:"useNativePlayer"`
	UsePost          bool             `json:"usePost"`
	UseLegacyParser  bool             `json:"useLegacyParser"`
	AdBlocker        bool             `json:"adBlocker"`
	UserAgent        string           `json:"userAgent"`
	BaseURL          string           `json:"baseURL"`
	SearchURL        string           `json:"searchURL"`
	SearchList       string           `json:"searchList"`
	SearchName       string           `json:"searchName"`
	SearchResult     string           `json:"searchResult"`
	ChapterRoads     string           `json:"chapterRoads"`
	ChapterResult    string           `json:"chapterResult"`
	Referer          string           `json:"referer"`
	SearchMode       string           `json:"searchMode"`
	ChapterMode      string           `json:"chapterMode"`
	SearchApiConfig  ApiSearchConfig  `json:"searchApiConfig,omitempty"`
	ChapterApiConfig ApiChapterConfig `json:"chapterApiConfig,omitempty"`
	Enabled          bool             `json:"enabled"`
}

type pluginAlias Plugin

func (p *Plugin) UnmarshalJSON(data []byte) error {
	type rawPlugin struct {
		pluginAlias
		Api any `json:"api"`
	}
	var r rawPlugin
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	*p = Plugin(r.pluginAlias)
	if r.Api != nil {
		p.Api = fmt.Sprintf("%v", r.Api)
	}
	return nil
}

type SearchItem struct {
	PluginName string `json:"pluginName"`
	Name       string `json:"name"`
	Src        string `json:"src"`
}

type Episode struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Road struct {
	Name       string    `json:"name"`
	Episodes   []Episode `json:"episodes"`
	Identifier []string  `json:"identifier"`
	Data       []string  `json:"data"`
}
