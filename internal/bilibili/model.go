package bilibili

// WbiImgKey holds the cached img_key and sub_key from Bilibili nav API
type WbiImgKey struct {
	ImgKey string `json:"img_key"`
	SubKey string `json:"sub_key"`
}

// NavResponse represents the response from /x/web-interface/nav
type NavResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		IsLogin       bool   `json:"isLogin"`
		Mid           int64  `json:"mid"`
		Uname         string `json:"uname"`
		Face          string `json:"face"`
		VipStatus     int    `json:"vipStatus"`     // 0: none, 1: valid
		VipType       int    `json:"vipType"`       // 1: monthly, 2: yearly
		VipDueDate    int64  `json:"vipDueDate"`    // timestamp ms
		VipStatusWarn string `json:"vipStatusWarn"`
		WbiImg        struct {
			ImgURL string `json:"img_url"`
			SubURL string `json:"sub_url"`
		} `json:"wbi_img"`
	} `json:"data"`
}

// QRGenerateResponse represents the response from /x/passport-login/web/qrcode/generate
type QRGenerateResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		URL       string `json:"url"`
		QRCodeKey string `json:"qrcode_key"`
	} `json:"data"`
}

// QRPollResponse represents the response from /x/passport-login/web/qrcode/poll
type QRPollResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		URL          string `json:"url"`
		RefreshToken string `json:"refresh_token"`
		Code         int    `json:"code"` // 0: success, 86101: not scanned, 86090: scanned not confirmed, 86038: expired
		Message      string `json:"message"`
	} `json:"data"`
}

// BilibiliCredentials stores user login credentials
type BilibiliCredentials struct {
	SessData   string `json:"sessdata"`
	BiliJct    string `json:"bili_jct"`
	Buvid3     string `json:"buvid3"`
	DedeUserID string `json:"dede_user_id"`
}

// BilibiliSettings stores user preference settings
type BilibiliSettings struct {
	Enabled        bool   `json:"enabled"`
	PreferBilibili bool   `json:"prefer_bilibili"`
	MaxQuality     int    `json:"max_quality"` // e.g. 120 (4K), 116 (1080P60), 80 (1080P), 64 (720P), 32 (480P)
	StreamMode     string `json:"stream_mode"` // "direct" (免ffmpeg MP4单流, default), "dash" (DASH混流需ffmpeg)
}

// BilibiliStatus represents user-facing status
type BilibiliStatus struct {
	IsLogin     bool   `json:"is_login"`
	Mid         int64  `json:"mid"`
	Uname       string `json:"uname"`
	Face        string `json:"face"`
	IsVip       bool   `json:"is_vip"`
	VipDueDate  int64  `json:"vip_due_date"`
	MaxQuality  int    `json:"max_quality"`
	QualityDesc string `json:"quality_desc"`
	StreamMode  string `json:"stream_mode"`
	Enabled     bool   `json:"enabled"`
	Prefer      bool   `json:"prefer"`
}

// DashStream represents DASH video or audio stream
type DashStream struct {
	ID        int      `json:"id"`
	BaseURL   string   `json:"baseUrl"`
	BackupURL []string `json:"backupUrl"`
	Bandwidth int      `json:"bandwidth"`
	Codecs    string   `json:"codecs"`
	Width     int      `json:"width"`
	Height    int      `json:"height"`
	FrameRate string   `json:"frameRate"`
}

// PlayURLResponse represents DASH playurl response
type PlayURLResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Result  struct {
		Quality          int      `json:"quality"`
		Format           string   `json:"format"`
		Timelength       int64    `json:"timelength"` // in ms
		AcceptQuality    []int    `json:"accept_quality"`
		AcceptDescription []string `json:"accept_description"`
		Dash             *DashData `json:"dash"`
		Durl             []struct {
			URL       string   `json:"url"`
			BackupURL []string `json:"backup_url"`
			Length    int64    `json:"length"`
			Size      int64    `json:"size"`
		} `json:"durl"`
	} `json:"result"`
	Data struct {
		Quality          int      `json:"quality"`
		Format           string   `json:"format"`
		Timelength       int64    `json:"timelength"`
		AcceptQuality    []int    `json:"accept_quality"`
		AcceptDescription []string `json:"accept_description"`
		Dash             *DashData `json:"dash"`
		Durl             []struct {
			URL       string   `json:"url"`
			BackupURL []string `json:"backup_url"`
			Length    int64    `json:"length"`
			Size      int64    `json:"size"`
		} `json:"durl"`
	} `json:"data"`
}

type DashData struct {
	Duration int          `json:"duration"`
	Video    []DashStream `json:"video"`
	Audio    []DashStream `json:"audio"`
	Dolby    *struct {
		Type  int          `json:"type"`
		Audio []DashStream `json:"audio"`
	} `json:"dolby"`
	Flac *struct {
		Audio *DashStream `json:"audio"`
	} `json:"flac"`
}

// SeasonResponse represents response from /pgc/view/web/season
type SeasonResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Result  struct {
		SeasonID  int    `json:"season_id"`
		SeasonTitle string `json:"season_title"`
		Title     string `json:"title"`
		Cover     string `json:"cover"`
		Episodes  []struct {
			ID       int    `json:"id"`
			Aid      int64  `json:"aid"`
			Bvid     string `json:"bvid"`
			Cid      int64  `json:"cid"`
			Title    string `json:"title"`
			LongTitle string `json:"long_title"`
			Badge    string `json:"badge"`
			ShareURL string `json:"share_url"`
		} `json:"episodes"`
	} `json:"result"`
}

// SearchPGCResponse represents search result from /x/web-interface/wbi/search/type?search_type=media_bangumi
type SearchPGCResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Result []struct {
			SeasonID int    `json:"season_id"`
			Title    string `json:"title"`
			OrgTitle string `json:"org_title"`
			Cover    string `json:"cover"`
			MediaType int   `json:"media_type"`
			SeasonType int  `json:"season_type"`
			Styles   string `json:"styles"`
			EPS []struct {
				ID       int    `json:"id"`
				Title    string `json:"title"`
				LongTitle string `json:"long_title"`
				IndexTitle string `json:"index_title"`
			} `json:"eps"`
		} `json:"result"`
	} `json:"data"`
}

// ResolvedBiliStream represents resolved playable stream info
type ResolvedBiliStream struct {
	Title        string `json:"title"`
	Quality      int    `json:"quality"`
	QualityLabel string `json:"quality_label"`
	Codec        string `json:"codec"`
	VideoURL     string `json:"video_url"`
	AudioURL     string `json:"audio_url"`
	IsDASH       bool   `json:"is_dash"`
	SingleURL    string `json:"single_url"` // If MP4/FLV single stream
	DurationMs   int64  `json:"duration_ms"`
	Referer      string `json:"referer"`
	UserAgent    string `json:"user_agent"`
}
