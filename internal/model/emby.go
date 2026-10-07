package model

import "time"

// PublicSystemInfo is returned by /emby/System/Info/Public
type PublicSystemInfo struct {
	LocalAddress        string `json:"LocalAddress,omitempty"`
	ServerName          string `json:"ServerName"`
	Version             string `json:"Version"`
	Id                  string `json:"Id"`
	OperatingSystem     string `json:"OperatingSystem,omitempty"`
	StartupWizardCompleted bool `json:"StartupWizardCompleted"`
}

// SystemInfo is returned by /emby/System/Info
type SystemInfo struct {
	SystemUpdateLevel             string   `json:"SystemUpdateLevel,omitempty"`
	OperatingSystemDisplayName   string   `json:"OperatingSystemDisplayName,omitempty"`
	PackageName                  string   `json:"PackageName,omitempty"`
	HasPendingRestart            bool     `json:"HasPendingRestart"`
	IsShuttingDown                bool     `json:"IsShuttingDown"`
	OperatingSystem              string   `json:"OperatingSystem"`
	SupportsLibraryMonitor       bool     `json:"SupportsLibraryMonitor"`
	WebSocketPortNumber          int      `json:"WebSocketPortNumber"`
	CompletedInstallations       []string `json:"CompletedInstallations"`
	CanSelfRestart               bool     `json:"CanSelfRestart"`
	CanSelfUpdate                bool     `json:"CanSelfUpdate"`
	CanLaunchWebBrowser          bool     `json:"CanLaunchWebBrowser"`
	ProgramDataPath              string   `json:"ProgramDataPath,omitempty"`
	ItemsByNamePath              string   `json:"ItemsByNamePath,omitempty"`
	CachePath                    string   `json:"CachePath,omitempty"`
	LogPath                      string   `json:"LogPath,omitempty"`
	InternalMetadataPath         string   `json:"InternalMetadataPath,omitempty"`
	TranscodingTempPath          string   `json:"TranscodingTempPath,omitempty"`
	HttpServerPortNumber         int      `json:"HttpServerPortNumber"`
	SupportsHttps                bool     `json:"SupportsHttps"`
	HttpsPortNumber              int      `json:"HttpsPortNumber,omitempty"`
	HasUpdateAvailable           bool     `json:"HasUpdateAvailable"`
	SupportsAutoRunAtStartup     bool     `json:"SupportsAutoRunAtStartup"`
	HardwareAccelerationRequiresPem bool `json:"HardwareAccelerationRequiresPem"`
	ServerName                   string   `json:"ServerName"`
	Version                      string   `json:"Version"`
	Id                           string   `json:"Id"`
}

// AuthenticateUserByNameRequest is the body for /emby/Users/AuthenticateByName
type AuthenticateUserByNameRequest struct {
	Username string `json:"Username"`
	Pw       string `json:"Pw"`
	Password string `json:"Password"`
}

// AuthenticationResult is returned upon successful authentication
type AuthenticationResult struct {
	User          UserDto `json:"User"`
	SessionInfo   any     `json:"SessionInfo,omitempty"`
	AccessToken   string  `json:"AccessToken"`
	ServerId      string  `json:"ServerId"`
}

type UserDto struct {
	Name                      string            `json:"Name"`
	ServerId                  string            `json:"ServerId"`
	ServerName                string            `json:"ServerName,omitempty"`
	Id                        string            `json:"Id"`
	HasPassword               bool              `json:"HasPassword"`
	HasConfiguredPassword     bool              `json:"HasConfiguredPassword"`
	HasConfiguredEasyPassword bool              `json:"HasConfiguredEasyPassword"`
	EnableAutoLogin           bool              `json:"EnableAutoLogin"`
	LastLoginDate             *time.Time        `json:"LastLoginDate,omitempty"`
	LastActivityDate          *time.Time        `json:"LastActivityDate,omitempty"`
	Configuration             UserConfiguration `json:"Configuration,omitempty"`
	Policy                    UserPolicy        `json:"Policy,omitempty"`
}

type UserConfiguration struct {
	PlayDefaultAudioTrack      bool     `json:"PlayDefaultAudioTrack"`
	SubtitleLanguagePreference string   `json:"SubtitleLanguagePreference"`
	DisplayMissingEpisodes     bool     `json:"DisplayMissingEpisodes"`
	EnableNextEpisodeAutoPlay  bool     `json:"EnableNextEpisodeAutoPlay"`
	GroupedFolders             []string `json:"GroupedFolders"`
	MyMediaExcludes            []string `json:"MyMediaExcludes"`
	OrderedViews               []string `json:"OrderedViews"`
	LatestItemsExcludes        []string `json:"LatestItemsExcludes"`
	HidePlayedInLatest         bool     `json:"HidePlayedInLatest"`
	RememberAudioSelections    bool     `json:"RememberAudioSelections"`
	RememberSubtitleSelections bool     `json:"RememberSubtitleSelections"`
}

type UserPolicy struct {
	IsAdministrator     bool     `json:"IsAdministrator"`
	IsDisabled          bool     `json:"IsDisabled"`
	IsHidden            bool     `json:"IsHidden"`
	EnableMediaPlayback bool     `json:"EnableMediaPlayback"`
	EnableAllDevices    bool     `json:"EnableAllDevices"`
	EnableAllChannels   bool     `json:"EnableAllChannels"`
	EnableAllFolders    bool     `json:"EnableAllFolders"`
	EnableAllLibraries  bool     `json:"EnableAllLibraries"`
	EnabledFolders      []string `json:"EnabledFolders"`
	EnabledLibraries    []string `json:"EnabledLibraries"`
	BlockedMediaFolders []string `json:"BlockedMediaFolders"`
	BlockedChannels     []string `json:"BlockedChannels"`
}

// BaseItemDto represents movies, series, seasons, episodes, views, etc.
type BaseItemDto struct {
	Name                    string            `json:"Name"`
	OriginalTitle           string            `json:"OriginalTitle,omitempty"`
	ServerId                string            `json:"ServerId,omitempty"`
	Id                      string            `json:"Id"`
	Type                    string            `json:"Type"` // UserView, Series, Season, Episode, CollectionFolder, AggregateFolder
	CollectionType          string            `json:"CollectionType,omitempty"` // tvshows, movies
	Overview                string            `json:"Overview,omitempty"`
	RunTimeTicks            int64             `json:"RunTimeTicks,omitempty"`
	ProductionYear          int               `json:"ProductionYear,omitempty"`
	IndexNumber             int               `json:"IndexNumber,omitempty"` // Episode number or Season number
	ParentIndexNumber       int               `json:"ParentIndexNumber,omitempty"` // Season number for episode
	SeriesName              string            `json:"SeriesName,omitempty"`
	SeriesId                string            `json:"SeriesId,omitempty"`
	SeasonName              string            `json:"SeasonName,omitempty"`
	SeasonId                string            `json:"SeasonId,omitempty"`
	CommunityRating         float64           `json:"CommunityRating,omitempty"`
	PremiereDate            string            `json:"PremiereDate,omitempty"`
	EndDate                 string            `json:"EndDate,omitempty"`
	DateCreated             string            `json:"DateCreated,omitempty"`
	Status                  string            `json:"Status,omitempty"`
	SortName                string            `json:"SortName,omitempty"`
	OfficialRating          string            `json:"OfficialRating,omitempty"`
	ChildCount              int               `json:"ChildCount,omitempty"`
	RecursiveItemCount      int               `json:"RecursiveItemCount,omitempty"`
	Genres                  []string          `json:"Genres,omitempty"`
	Tags                    []string          `json:"Tags,omitempty"`
	ProviderIds             map[string]string `json:"ProviderIds,omitempty"`
	ImageTags               map[string]string `json:"ImageTags,omitempty"`
	BackdropImageTags       []string          `json:"BackdropImageTags,omitempty"`
	PrimaryImageAspectRatio float64           `json:"PrimaryImageAspectRatio,omitempty"`
	DisplayPreferencesId    string            `json:"DisplayPreferencesId,omitempty"`
	ChannelId               *string           `json:"ChannelId,omitempty"`
	UserData                *UserItemDataDto  `json:"UserData,omitempty"`
	MediaType               string            `json:"MediaType,omitempty"` // Video
	Container               string            `json:"Container,omitempty"` // mp4
	Path                    string            `json:"Path,omitempty"`
	MediaSources            []MediaSourceInfo `json:"MediaSources,omitempty"`
	MediaStreams            []MediaStream     `json:"MediaStreams,omitempty"`
	PlayAccess              string            `json:"PlayAccess,omitempty"` // Full
	CanPlay                 bool              `json:"CanPlay"`
	CanResume               bool              `json:"CanResume"`
	IsPlaceHolder           bool              `json:"IsPlaceHolder"`
	IsFolder                bool              `json:"IsFolder"`
	LocationType            string            `json:"LocationType,omitempty"` // FileSystem, Virtual
	Guid                    string            `json:"Guid,omitempty"`
	PresentationUniqueKey   string            `json:"PresentationUniqueKey,omitempty"`
	EnableMediaSourceDisplay bool             `json:"EnableMediaSourceDisplay,omitempty"`
	SortIndexNumber         int               `json:"SortIndexNumber,omitempty"`
	CanDownload             bool              `json:"CanDownload"`
	SupportsSync            bool              `json:"SupportsSync"`
}

type UserItemDataDto struct {
	Rating               float64 `json:"Rating,omitempty"`
	PlayedPercentage     float64 `json:"PlayedPercentage,omitempty"`
	UnplayedItemCount    int     `json:"UnplayedItemCount,omitempty"`
	PlaybackPositionTicks int64  `json:"PlaybackPositionTicks"`
	PlayCount            int     `json:"PlayCount"`
	IsFavorite           bool    `json:"IsFavorite"`
	Likes                *bool   `json:"Likes,omitempty"`
	LastPlayedDate       string  `json:"LastPlayedDate,omitempty"`
	Played               bool    `json:"Played"`
	Key                  string  `json:"Key,omitempty"`
}

type QueryResult[T any] struct {
	Items            []T `json:"Items"`
	TotalRecordCount int `json:"TotalRecordCount"`
}

func NewQueryResult[T any](items []T) QueryResult[T] {
	if items == nil {
		items = make([]T, 0)
	}
	return QueryResult[T]{
		Items:            items,
		TotalRecordCount: len(items),
	}
}

type MediaSourceInfo struct {
	Id                    string        `json:"Id"`
	Name                  string        `json:"Name,omitempty"`
	Path                  string        `json:"Path,omitempty"`
	DirectStreamUrl       string        `json:"DirectStreamUrl,omitempty"`
	Protocol              string        `json:"Protocol,omitempty"` // Http
	Container             string        `json:"Container,omitempty"` // hls, mp4
	Type                  string        `json:"Type,omitempty"` // Default
	Bitrate               int           `json:"Bitrate,omitempty"`
	RunTimeTicks          int64         `json:"RunTimeTicks,omitempty"`
	IsRemote              bool          `json:"IsRemote"`
	SupportsDirectStream  bool          `json:"SupportsDirectStream"`
	SupportsDirectPlay    bool          `json:"SupportsDirectPlay"`
	SupportsTranscoding   bool          `json:"SupportsTranscoding"`
	MediaStreams          []MediaStream `json:"MediaStreams,omitempty"`
}

type MediaStream struct {
	Codec          string `json:"Codec,omitempty"`
	Language       string `json:"Language,omitempty"`
	DisplayTitle   string `json:"DisplayTitle,omitempty"`
	Type           string `json:"Type"` // Video, Audio, Subtitle
	Index          int    `json:"Index"`
	Width          int    `json:"Width,omitempty"`
	Height         int    `json:"Height,omitempty"`
	BitRate        int    `json:"BitRate,omitempty"`
	IsAVC          bool   `json:"IsAVC,omitempty"`
	IsExternal     bool   `json:"IsExternal,omitempty"`
	DeliveryMethod string `json:"DeliveryMethod,omitempty"` // External, Embed
	DeliveryUrl    string `json:"DeliveryUrl,omitempty"`
	IsDefault      bool   `json:"IsDefault,omitempty"`
}

type PlaybackInfoResponse struct {
	MediaSources  []MediaSourceInfo `json:"MediaSources"`
	PlaySessionId string            `json:"PlaySessionId,omitempty"`
}
