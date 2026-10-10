package bilibili

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36"
	defaultReferer   = "https://www.bilibili.com/"
	apiBaseURL       = "https://api.bilibili.com"
	passportBaseURL  = "https://passport.bilibili.com"
)

type Client struct {
	httpClient *http.Client
	wbiSigner  *WbiSigner
	userAgent  string
}

func NewClient(wbiSigner *WbiSigner) *Client {
	if wbiSigner == nil {
		wbiSigner = NewWbiSigner()
	}
	return &Client{
		wbiSigner: wbiSigner,
		userAgent: defaultUserAgent,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
				DisableKeepAlives: false,
				MaxIdleConns:      50,
				IdleConnTimeout:   90 * time.Second,
			},
		},
	}
}

// GetSigner returns the internal WbiSigner
func (c *Client) GetSigner() *WbiSigner {
	return c.wbiSigner
}

func (c *Client) buildCookieHeader(creds *BilibiliCredentials) string {
	if creds == nil {
		return ""
	}
	var parts []string
	if creds.SessData != "" {
		parts = append(parts, "SESSDATA="+strings.TrimSpace(creds.SessData))
	}
	if creds.BiliJct != "" {
		parts = append(parts, "bili_jct="+strings.TrimSpace(creds.BiliJct))
	}
	if creds.Buvid3 != "" {
		parts = append(parts, "buvid3="+strings.TrimSpace(creds.Buvid3))
	}
	if creds.DedeUserID != "" {
		parts = append(parts, "DedeUserID="+strings.TrimSpace(creds.DedeUserID))
	}
	return strings.Join(parts, "; ")
}

// FetchNav queries /x/web-interface/nav to get user profile and update WBI keys
func (c *Client) FetchNav(ctx context.Context, creds *BilibiliCredentials) (*NavResponse, error) {
	reqURL := apiBaseURL + "/x/web-interface/nav"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Referer", defaultReferer)

	cookie := c.buildCookieHeader(creds)
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nav request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read nav response: %w", err)
	}

	var nav NavResponse
	if err := json.Unmarshal(body, &nav); err != nil {
		return nil, fmt.Errorf("failed to parse nav json: %w", err)
	}

	// Update WBI keys if present
	if nav.Data.WbiImg.ImgURL != "" && nav.Data.WbiImg.SubURL != "" {
		c.wbiSigner.UpdateFromURLs(nav.Data.WbiImg.ImgURL, nav.Data.WbiImg.SubURL)
		log.Printf("[Bilibili] Updated WBI keys: %s", c.wbiSigner)
	}

	return &nav, nil
}

// EnsureWbiKeys checks and refreshes WBI keys if necessary
func (c *Client) EnsureWbiKeys(ctx context.Context, creds *BilibiliCredentials) error {
	if c.wbiSigner.NeedsRefresh() {
		_, err := c.FetchNav(ctx, creds)
		return err
	}
	return nil
}

// SearchPGC searches bangumi / anime / movie using WBI signed query
func (c *Client) SearchPGC(ctx context.Context, keyword string, searchType string, creds *BilibiliCredentials) (*SearchPGCResponse, error) {
	_ = c.EnsureWbiKeys(ctx, creds)
	if searchType == "" {
		searchType = "media_bangumi"
	}

	params := map[string]string{
		"keyword":     keyword,
		"search_type": searchType,
		"page":        "1",
		"page_size":   "20",
	}

	signedQuery := c.wbiSigner.EncodeSignedQuery(params)
	reqURL := apiBaseURL + "/x/web-interface/wbi/search/type?" + signedQuery

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Referer", defaultReferer)

	cookie := c.buildCookieHeader(creds)
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search PGC request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read search response: %w", err)
	}

	var res SearchPGCResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("failed to parse search json: %w", err)
	}

	return &res, nil
}

// GetSeason fetches bangumi season details & episode list
func (c *Client) GetSeason(ctx context.Context, seasonID int, epID int, creds *BilibiliCredentials) (*SeasonResponse, error) {
	u, _ := url.Parse(apiBaseURL + "/pgc/view/web/season")
	q := u.Query()
	if seasonID > 0 {
		q.Set("season_id", strconv.Itoa(seasonID))
	}
	if epID > 0 {
		q.Set("ep_id", strconv.Itoa(epID))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Referer", defaultReferer)

	cookie := c.buildCookieHeader(creds)
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get season request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read season response: %w", err)
	}

	var res SeasonResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("failed to parse season json: %w", err)
	}

	return &res, nil
}

// GetPGCPlayURL fetches play URL for PGC anime episodes (fnval=0 for MP4 single stream, fnval=4048 for DASH)
func (c *Client) GetPGCPlayURL(ctx context.Context, epID int, qn int, fnval int, creds *BilibiliCredentials) (*PlayURLResponse, error) {
	if qn <= 0 {
		qn = 120 // Request highest permissible quality by default (120=4K, 112=1080P+, 80=1080P)
	}

	params := url.Values{}
	params.Set("ep_id", strconv.Itoa(epID))
	params.Set("qn", strconv.Itoa(qn))
	params.Set("fnval", strconv.Itoa(fnval))
	params.Set("fnver", "0")
	params.Set("fourk", "1")

	reqURL := apiBaseURL + "/pgc/player/web/playurl?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Referer", fmt.Sprintf("https://www.bilibili.com/bangumi/play/ep%d", epID))

	cookie := c.buildCookieHeader(creds)
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("PGC playurl request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read PGC playurl response: %w", err)
	}

	var playRes PlayURLResponse
	if err := json.Unmarshal(body, &playRes); err != nil {
		return nil, fmt.Errorf("failed to parse playurl json: %w", err)
	}

	return &playRes, nil
}

// GetUGCPlayURL fetches play URL for user submitted videos (bvid + cid)
func (c *Client) GetUGCPlayURL(ctx context.Context, bvid string, cid int64, qn int, fnval int, creds *BilibiliCredentials) (*PlayURLResponse, error) {
	_ = c.EnsureWbiKeys(ctx, creds)

	if qn <= 0 {
		qn = 120 // Request highest permissible quality by default (120=4K, 112=1080P+, 80=1080P)
	}

	params := map[string]string{
		"bvid":  bvid,
		"cid":   strconv.FormatInt(cid, 10),
		"qn":    strconv.Itoa(qn),
		"fnval": strconv.Itoa(fnval),
		"fnver": "0",
		"fourk": "1",
	}

	signedQuery := c.wbiSigner.EncodeSignedQuery(params)
	reqURL := apiBaseURL + "/x/player/wbi/playurl?" + signedQuery

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Referer", fmt.Sprintf("https://www.bilibili.com/video/%s", bvid))

	cookie := c.buildCookieHeader(creds)
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("UGC playurl request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read UGC playurl response: %w", err)
	}

	var playRes PlayURLResponse
	if err := json.Unmarshal(body, &playRes); err != nil {
		return nil, fmt.Errorf("failed to parse playurl json: %w", err)
	}

	return &playRes, nil
}

// GenerateQRCode creates a QR code login session
func (c *Client) GenerateQRCode(ctx context.Context) (*QRGenerateResponse, error) {
	reqURL := passportBaseURL + "/x/passport-login/web/qrcode/generate"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Referer", defaultReferer)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("generate QR request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read QR response: %w", err)
	}

	var qr QRGenerateResponse
	if err := json.Unmarshal(body, &qr); err != nil {
		return nil, fmt.Errorf("failed to parse QR json: %w", err)
	}

	return &qr, nil
}

// PollQRCode polls the login state of the QR code
func (c *Client) PollQRCode(ctx context.Context, qrcodeKey string) (*QRPollResponse, *BilibiliCredentials, error) {
	reqURL := fmt.Sprintf("%s/x/passport-login/web/qrcode/poll?qrcode_key=%s", passportBaseURL, url.QueryEscape(qrcodeKey))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Referer", defaultReferer)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("poll QR request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read poll QR response: %w", err)
	}

	var poll QRPollResponse
	if err := json.Unmarshal(body, &poll); err != nil {
		return nil, nil, fmt.Errorf("failed to parse poll QR json: %w", err)
	}

	var creds *BilibiliCredentials
	if poll.Data.Code == 0 || poll.Code == 0 {
		creds = &BilibiliCredentials{}

		// 1. Extract from standard Cookies
		for _, cookie := range resp.Cookies() {
			switch cookie.Name {
			case "SESSDATA":
				if creds.SessData == "" {
					creds.SessData = cookie.Value
				}
			case "bili_jct":
				if creds.BiliJct == "" {
					creds.BiliJct = cookie.Value
				}
			case "buvid3":
				if creds.Buvid3 == "" {
					creds.Buvid3 = cookie.Value
				}
			case "DedeUserID":
				if creds.DedeUserID == "" {
					creds.DedeUserID = cookie.Value
				}
			}
		}

		// 2. Extract from raw Set-Cookie response headers
		if creds.SessData == "" {
			for _, h := range resp.Header["Set-Cookie"] {
				parts := strings.Split(h, ";")
				for _, part := range parts {
					kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
					if len(kv) == 2 {
						switch kv[0] {
						case "SESSDATA":
							if creds.SessData == "" {
								creds.SessData = kv[1]
							}
						case "bili_jct":
							if creds.BiliJct == "" {
								creds.BiliJct = kv[1]
							}
						case "buvid3":
							if creds.Buvid3 == "" {
								creds.Buvid3 = kv[1]
							}
						case "DedeUserID":
							if creds.DedeUserID == "" {
								creds.DedeUserID = kv[1]
							}
						}
					}
				}
			}
		}

		// 3. Extract from redirect / crossDomain URL query parameters in response body
		if poll.Data.URL != "" {
			if parsedURL, err := url.Parse(poll.Data.URL); err == nil {
				q := parsedURL.Query()
				if creds.SessData == "" && q.Get("SESSDATA") != "" {
					creds.SessData = q.Get("SESSDATA")
				}
				if creds.BiliJct == "" && q.Get("bili_jct") != "" {
					creds.BiliJct = q.Get("bili_jct")
				}
				if creds.DedeUserID == "" && q.Get("DedeUserID") != "" {
					creds.DedeUserID = q.Get("DedeUserID")
				}
			}
		}
	}

	return &poll, creds, nil
}

// GetStatus computes user-facing status based on credentials and settings
func (c *Client) GetStatus(ctx context.Context, creds *BilibiliCredentials, settings *BilibiliSettings) *BilibiliStatus {
	st := &BilibiliStatus{
		IsLogin:     false,
		MaxQuality:  32,
		QualityDesc: "480P 清晰 (免登录)",
		Enabled:     true,
		Prefer:      false,
	}

	if settings != nil {
		st.Enabled = settings.Enabled
		st.Prefer = settings.PreferBilibili
		st.StreamMode = settings.StreamMode
	}
	if st.StreamMode == "" {
		st.StreamMode = "direct"
	}

	if creds == nil || creds.SessData == "" {
		return st
	}

	nav, err := c.FetchNav(ctx, creds)
	if err != nil || nav == nil || nav.Code != 0 || !nav.Data.IsLogin {
		return st
	}

	st.IsLogin = true
	st.Mid = nav.Data.Mid
	st.Uname = nav.Data.Uname
	st.Face = nav.Data.Face
	st.IsVip = nav.Data.VipStatus == 1
	st.VipDueDate = nav.Data.VipDueDate

	if st.IsVip {
		st.MaxQuality = 120
		st.QualityDesc = "4K 超清 / 杜比视界 (大会员)"
	} else {
		st.MaxQuality = 80
		st.QualityDesc = "1080P 高清 (普通登录)"
	}

	return st
}
