package bilibili

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var mixinKeyEncTab = []int{
	46, 47, 18, 2, 53, 8, 23, 32, 29, 50, 11, 4, 9, 13, 22, 37,
	45, 56, 12, 60, 63, 28, 41, 57, 5, 49, 10, 44, 30, 39, 36, 17,
	19, 21, 51, 15, 42, 16, 31, 62, 40, 55, 25, 24, 35, 43, 58, 61,
	26, 33, 59, 6, 38, 27, 48, 1, 34, 47, 52, 3, 20, 14, 54, 7,
}

type WbiSigner struct {
	mu        sync.RWMutex
	imgKey    string
	subKey    string
	updatedAt time.Time
}

func NewWbiSigner() *WbiSigner {
	return &WbiSigner{}
}

// UpdateKeys sets new img_key and sub_key extracted from nav response
func (w *WbiSigner) UpdateKeys(imgKey, subKey string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.imgKey = imgKey
	w.subKey = subKey
	w.updatedAt = time.Now()
}

// UpdateFromURLs extracts img_key and sub_key from full image URLs
func (w *WbiSigner) UpdateFromURLs(imgURL, subURL string) {
	imgKey := extractKeyFromURL(imgURL)
	subKey := extractKeyFromURL(subURL)
	if imgKey != "" && subKey != "" {
		w.UpdateKeys(imgKey, subKey)
	}
}

// NeedsRefresh returns true if keys are empty or older than 6 hours
func (w *WbiSigner) NeedsRefresh() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.imgKey == "" || w.subKey == "" || time.Since(w.updatedAt) > 6*time.Hour
}

// HasKeys returns true if keys are currently available
func (w *WbiSigner) HasKeys() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.imgKey != "" && w.subKey != ""
}

func extractKeyFromURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	base := path.Base(u.Path)
	ext := path.Ext(base)
	return strings.TrimSuffix(base, ext)
}

func getMixinKey(imgKey, subKey string) string {
	rawWbiKey := imgKey + subKey
	var mixinKey strings.Builder
	for _, idx := range mixinKeyEncTab {
		if idx < len(rawWbiKey) {
			mixinKey.WriteByte(rawWbiKey[idx])
		}
	}
	res := mixinKey.String()
	if len(res) > 32 {
		return res[:32]
	}
	return res
}

// SignParams calculates and injects wts and w_rid into the given query parameters
func (w *WbiSigner) SignParams(params map[string]string) map[string]string {
	w.mu.RLock()
	imgKey := w.imgKey
	subKey := w.subKey
	w.mu.RUnlock()

	if imgKey == "" || subKey == "" {
		// Fallback keys if not yet fetched
		imgKey = "653657f524a14777892523e197217b51"
		subKey = "e606345e317b4c7384a6cfa3e9e30a59"
	}

	result := make(map[string]string, len(params)+2)
	for k, v := range params {
		result[k] = v
	}

	if _, ok := result["wts"]; !ok {
		result["wts"] = strconv.FormatInt(time.Now().Unix(), 10)
	}

	mixinKey := getMixinKey(imgKey, subKey)

	// Sort keys
	keys := make([]string, 0, len(result))
	for k := range result {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Filter out invalid chars & build query
	var query strings.Builder
	first := true
	for _, k := range keys {
		val := result[k]
		// Sanitize value for WBI
		val = sanitizeWbiValue(val)
		if !first {
			query.WriteByte('&')
		}
		first = false
		query.WriteString(url.QueryEscape(k))
		query.WriteByte('=')
		query.WriteString(url.QueryEscape(val))
	}

	sum := md5.Sum([]byte(query.String() + mixinKey))
	wRid := hex.EncodeToString(sum[:])
	result["w_rid"] = wRid

	return result
}

// EncodeSignedQuery builds a full URL query string from signed params
func (w *WbiSigner) EncodeSignedQuery(params map[string]string) string {
	signed := w.SignParams(params)
	keys := make([]string, 0, len(signed))
	for k := range signed {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var query strings.Builder
	for i, k := range keys {
		if i > 0 {
			query.WriteByte('&')
		}
		query.WriteString(url.QueryEscape(k))
		query.WriteByte('=')
		query.WriteString(url.QueryEscape(signed[k]))
	}
	return query.String()
}

func sanitizeWbiValue(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '!', '\'', '(', ')', '*':
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// CalculateMixinKey is exported for testing
func CalculateMixinKey(imgKey, subKey string) string {
	return getMixinKey(imgKey, subKey)
}

// SignWithKeys is a helper for direct calculation
func SignWithKeys(params map[string]string, imgKey, subKey string) map[string]string {
	signer := &WbiSigner{
		imgKey:    imgKey,
		subKey:    subKey,
		updatedAt: time.Now(),
	}
	return signer.SignParams(params)
}

// VerifyWbiSigner tests WbiSigner validity
func (w *WbiSigner) String() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return fmt.Sprintf("WbiSigner(imgKey=%s, subKey=%s, age=%s)", w.imgKey, w.subKey, time.Since(w.updatedAt).Round(time.Second))
}
