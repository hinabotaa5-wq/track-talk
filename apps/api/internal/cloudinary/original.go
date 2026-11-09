package cloudinary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OriginalTag は、縮めずに保存した動画（VideoAsyncPreset で上がったもの）に付く印。
// 元ファイルには撮影日時・位置情報・端末情報が残り、容量も大きいので、配信用の派生ができたら
// 縮めた版で置き換え（ShrinkOriginal）、この印を外す。
const OriginalTag = "tt-original"

// replaceTimeout は、派生を元ファイルとして取り込み直すアップロードの待ち時間。Cloudinary が URL から取りにいくので、通常の API より長く待つ。
const replaceTimeout = 2 * time.Minute

// ErrNotOriginal は、その動画が置き換えの対象ではない（印が無い、置き換え済み、古い画面で縮めて上げた）ことを表す。
var ErrNotOriginal = errors.New("not an original video")

// OriginalAsset は、置き換えを待つ動画の1件。
type OriginalAsset struct {
	PublicID string `json:"public_id"`
	Type     string `json:"type"`
}

type videoKind struct {
	deliveryType string
	derived      []derivedSpec
	source       func(Client, string) string
}

func kindOf(publicID string) (videoKind, bool) {
	switch {
	case ValidPublicID(publicID):
		return videoKind{"upload", publicVideoDerived, Client.PlaybackURL}, true
	case ValidCoachingPublicID(publicID):
		return videoKind{CoachingVideoType, coachingVideoDerived, Client.CoachingPlaybackURL}, true
	}
	return videoKind{}, false
}

// ShrinkOriginal は、縮めずに保存した動画を、作り終えた再生用の派生（長辺1280px・q_auto の mp4）で置き換える。
// public_id と配信 URL は変わらない。置き換えると元ファイルとそのメタデータは Cloudinary に残らない。
// 再生用の派生がまだなら ErrProcessing、対象でなければ ErrNotOriginal を返す。
func (c Client) ShrinkOriginal(ctx context.Context, publicID string) error {
	kind, ok := kindOf(publicID)
	if !ok {
		return ErrInvalid
	}
	if !c.Configured() {
		return ErrNotConfigured
	}
	info, err := c.videoDetails(ctx, kind.deliveryType, publicID)
	if err != nil {
		return err
	}
	if !hasTag(info.Tags, OriginalTag) {
		return ErrNotOriginal
	}
	if !hasDerived(info.Derived, kind.derived[0]) {
		return ErrProcessing
	}
	source := kind.source(c, publicID)
	if source == "" {
		return ErrNotConfigured
	}
	// 置き換えた後も、配信で使う派生（再生・サムネイル）を裏で作り直させる。
	if err := c.signedPost(ctx, "/video/upload", map[string]string{
		"public_id":   publicID,
		"type":        kind.deliveryType,
		"overwrite":   "true",
		"invalidate":  "true",
		"eager":       eagerFor(kind.derived),
		"eager_async": eagerAsyncParameter,
	}, map[string]string{"file": source}); err != nil {
		return fmt.Errorf("replace original: %w", err)
	}
	// 置き換えで印が消えない場合に備え、明示的に外す（消えていれば何も起きない）。
	if err := c.signedPost(ctx, "/video/tags", map[string]string{
		"command":    "remove",
		"tag":        OriginalTag,
		"public_ids": publicID,
		"type":       kind.deliveryType,
	}, nil); err != nil {
		return fmt.Errorf("remove original tag: %w", err)
	}
	return nil
}

// ListOriginals は、まだ置き換えていない動画を1ページ分返す。次のページが無ければ cursor は空。
func (c Client) ListOriginals(ctx context.Context, cursor string) ([]OriginalAsset, string, error) {
	if !c.Configured() {
		return nil, "", ErrNotConfigured
	}
	values := url.Values{"max_results": {"100"}}
	if cursor != "" {
		values.Set("next_cursor", cursor)
	}
	endpoint := c.base() + "/v1_1/" + url.PathEscape(c.Cloud) + "/resources/video/tags/" + url.PathEscape(OriginalTag) + "?" + values.Encode()
	var page struct {
		Resources  []OriginalAsset `json:"resources"`
		NextCursor string          `json:"next_cursor"`
	}
	if err := c.adminGet(ctx, endpoint, &page); err != nil {
		return nil, "", err
	}
	return page.Resources, page.NextCursor, nil
}

type videoInfo struct {
	Tags    []string  `json:"tags"`
	Derived []derived `json:"derived"`
}

func (c Client) videoDetails(ctx context.Context, deliveryType, publicID string) (videoInfo, error) {
	var info videoInfo
	endpoint := c.base() + "/v1_1/" + url.PathEscape(c.Cloud) + "/resources/video/" + deliveryType + "/" + deliveryPath(publicID)
	err := c.adminGet(ctx, endpoint, &info)
	return info, err
}

func (c Client) adminGet(ctx context.Context, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ErrUnavailable
	}
	req.SetBasicAuth(c.Key, c.Secret)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return ErrUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return ErrUnavailable
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out); err != nil {
		return ErrUnavailable
	}
	return nil
}

// signedPost は Upload API へ署名付きで送る。unsigned の値（file など）は署名に含めない。
func (c Client) signedPost(ctx context.Context, path string, params, unsigned map[string]string) error {
	params["timestamp"] = fmt.Sprintf("%d", c.now().Unix())
	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}
	values.Set("signature", Sign(params, c.Secret))
	values.Set("api_key", c.Key)
	for k, v := range unsigned {
		values.Set(k, v)
	}
	ctx, cancel := context.WithTimeout(ctx, replaceTimeout)
	defer cancel()
	endpoint := c.base() + "/v1_1/" + url.PathEscape(c.Cloud) + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return ErrUnavailable
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: replaceTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return ErrUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("%w: %d %s", ErrUnavailable, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (c Client) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

func hasDerived(have []derived, want derivedSpec) bool {
	for _, d := range have {
		if sameDerived(d, want) {
			return true
		}
	}
	return false
}
