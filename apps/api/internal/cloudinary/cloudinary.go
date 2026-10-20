package cloudinary

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	Folder        = "track-talk"
	ImagePreset   = "track-talk-image"
	VideoPreset   = "track-talk-video"
	ImageFormats  = "gif,heic,heif,jpg,png,webp"
	VideoFormats  = "mov,mp4,webm"
	MaxImageBytes = 10 << 20
	MaxVideoBytes = 100 << 20

	// 添削の動画は顔や体が写るので、公開 URL では配信しない（type=authenticated）。
	// 署名付きの URL を知っている人だけが見られる。
	CoachingFolder      = "track-talk-coaching"
	CoachingVideoType   = "authenticated"
	MaxCoachingSeconds  = 30
	coachingSecondsSlop = 0.5

	// 保存する前に Cloudinary 側で縮める。元ファイルは残らない。
	// 写真は表示(長辺1600px)より少し大きい2048pxまで、動画は720p相当(長辺1280px)まで。
	ImageIncoming = "c_limit,w_2048,h_2048"
	VideoIncoming = "c_limit,w_1280,h_1280,q_auto"
)

var (
	ErrNotConfigured = errors.New("cloudinary not configured")
	ErrInvalid       = errors.New("invalid public id")
	ErrNotFound      = errors.New("video not found")
	ErrRejected      = errors.New("asset rejected")
	ErrTooLarge      = errors.New("asset too large")
	ErrTooLong       = errors.New("video too long")
	ErrUnavailable   = errors.New("cloudinary unavailable")
)

type Client struct {
	Cloud   string
	Key     string
	Secret  string
	APIBase string
	HTTP    *http.Client
	// AuthTokenKey は Cloudinary のトークン認証の鍵（16進数）。あれば非公開の動画の URL に期限を付ける。
	// 空のあいだは、期限のない署名付き URL のまま。
	AuthTokenKey string
	// Now は期限の起点。テストで差し替える。nil なら time.Now。
	Now func() time.Time
}

// AuthTokenTTL は、非公開の動画の URL を使える時間。画面を開き直せば新しい URL を受け取れる。
const AuthTokenTTL = 3 * time.Hour

func (c Client) Configured() bool {
	return c.Cloud != "" && c.Key != "" && c.Secret != ""
}

func (c Client) base() string {
	if c.APIBase != "" {
		return strings.TrimRight(c.APIBase, "/")
	}
	return "https://api.cloudinary.com"
}

func (c Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 10 * time.Second}
}

type UploadSignature struct {
	CloudName      string `json:"cloud_name"`
	APIKey         string `json:"api_key"`
	Timestamp      int64  `json:"timestamp"`
	PublicID       string `json:"public_id"`
	Signature      string `json:"signature"`
	UploadPreset   string `json:"upload_preset"`
	AllowedFormats string `json:"allowed_formats"`
	ResourceType   string `json:"resource_type"`
	Type           string `json:"type,omitempty"`
}

func NewPublicID() string {
	return Folder + "/" + uuid.NewString()
}

func ValidPublicID(publicID string) bool {
	rest, ok := strings.CutPrefix(publicID, Folder+"/")
	if !ok || strings.Contains(rest, "/") {
		return false
	}
	_, err := uuid.Parse(rest)
	return err == nil
}

func NewCoachingPublicID() string {
	return CoachingFolder + "/" + uuid.NewString()
}

func ValidCoachingPublicID(publicID string) bool {
	rest, ok := strings.CutPrefix(publicID, CoachingFolder+"/")
	if !ok || strings.Contains(rest, "/") {
		return false
	}
	_, err := uuid.Parse(rest)
	return err == nil
}

func Sign(params map[string]string, secret string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}
	sum := sha1.Sum([]byte(strings.Join(parts, "&") + secret))
	return hex.EncodeToString(sum[:])
}

func presetFor(resource string) (preset, formats string, maxBytes int64, err error) {
	switch resource {
	case "image":
		return ImagePreset, ImageFormats, MaxImageBytes, nil
	case "video":
		return VideoPreset, VideoFormats, MaxVideoBytes, nil
	default:
		return "", "", 0, ErrInvalid
	}
}

func (c Client) SignedUpload(now time.Time, resource string) (UploadSignature, error) {
	if !c.Configured() {
		return UploadSignature{}, ErrNotConfigured
	}
	preset, formats, _, err := presetFor(resource)
	if err != nil {
		return UploadSignature{}, err
	}
	publicID := NewPublicID()
	ts := now.Unix()
	params := map[string]string{
		"allowed_formats": formats,
		"public_id":       publicID,
		"timestamp":       fmt.Sprintf("%d", ts),
		"upload_preset":   preset,
	}
	return UploadSignature{
		CloudName:      c.Cloud,
		APIKey:         c.Key,
		Timestamp:      ts,
		PublicID:       publicID,
		Signature:      Sign(params, c.Secret),
		UploadPreset:   preset,
		AllowedFormats: formats,
		ResourceType:   resource,
	}, nil
}

// SignedCoachingUpload は添削の動画を非公開（authenticated）でアップロードする署名を返す。
// type も署名に含めるので、ブラウザ側で公開に書き換えると Cloudinary が受け付けない。
func (c Client) SignedCoachingUpload(now time.Time) (UploadSignature, error) {
	if !c.Configured() {
		return UploadSignature{}, ErrNotConfigured
	}
	publicID := NewCoachingPublicID()
	ts := now.Unix()
	params := map[string]string{
		"allowed_formats": VideoFormats,
		"public_id":       publicID,
		"timestamp":       fmt.Sprintf("%d", ts),
		"type":            CoachingVideoType,
		"upload_preset":   VideoPreset,
	}
	return UploadSignature{
		CloudName:      c.Cloud,
		APIKey:         c.Key,
		Timestamp:      ts,
		PublicID:       publicID,
		Signature:      Sign(params, c.Secret),
		UploadPreset:   VideoPreset,
		AllowedFormats: VideoFormats,
		ResourceType:   "video",
		Type:           CoachingVideoType,
	}, nil
}

func (c Client) EnsureUploadPresets(ctx context.Context) error {
	if !c.Configured() {
		return nil
	}
	for _, resource := range []string{"image", "video"} {
		preset, formats, _, err := presetFor(resource)
		if err != nil {
			return err
		}
		if err := c.ensurePreset(ctx, preset, formats, incomingFor(resource)); err != nil {
			return err
		}
	}
	return nil
}

func incomingFor(resource string) string {
	if resource == "video" {
		return VideoIncoming
	}
	return ImageIncoming
}

func (c Client) ensurePreset(ctx context.Context, name, formats, transformation string) error {
	values := url.Values{
		"name":            {name},
		"unsigned":        {"false"},
		"allowed_formats": {formats},
		"overwrite":       {"false"},
		"unique_filename": {"false"},
		"transformation":  {transformation},
	}
	endpoint := c.base() + "/v1_1/" + url.PathEscape(c.Cloud) + "/upload_presets"
	status, body, err := c.form(ctx, http.MethodPost, endpoint, values)
	if err != nil {
		return err
	}
	if status >= 200 && status < 300 {
		return nil
	}
	if status != http.StatusBadRequest && status != http.StatusConflict && !bytes.Contains(bytes.ToLower(body), []byte("exist")) {
		return fmt.Errorf("%w: create preset %s (%d)", ErrUnavailable, name, status)
	}
	delete(values, "name")
	status, _, err = c.form(ctx, http.MethodPut, endpoint+"/"+url.PathEscape(name), values)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("%w: update preset %s (%d)", ErrUnavailable, name, status)
	}
	return nil
}

func (c Client) form(ctx context.Context, method, endpoint string, values url.Values) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return 0, nil, ErrUnavailable
	}
	req.SetBasicAuth(c.Key, c.Secret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return 0, nil, ErrUnavailable
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, nil, ErrUnavailable
	}
	return resp.StatusCode, body, nil
}

func (c Client) ThumbnailURL(publicID string) string {
	if c.Cloud == "" || publicID == "" {
		return ""
	}
	return "https://res.cloudinary.com/" + url.PathEscape(c.Cloud) + "/video/upload/so_auto,c_limit,w_800,f_jpg/" + deliveryPath(publicID) + ".jpg"
}

func (c Client) PlaybackURL(publicID string) string {
	if c.Cloud == "" || publicID == "" {
		return ""
	}
	return "https://res.cloudinary.com/" + url.PathEscape(c.Cloud) + "/video/upload/c_limit,w_1280,h_1280,q_auto/" + deliveryPath(publicID) + ".mp4"
}

func (c Client) ImageURL(publicID string) string {
	if c.Cloud == "" || publicID == "" {
		return ""
	}
	return "https://res.cloudinary.com/" + url.PathEscape(c.Cloud) + "/image/upload/c_limit,w_1600,q_auto,f_auto/" + deliveryPath(publicID)
}

// AvatarURL は顔写真を正方形に切り抜いた URL を返す。顔が真ん中に来るように切る。
func (c Client) AvatarURL(publicID string) string {
	if c.Cloud == "" || publicID == "" {
		return ""
	}
	return "https://res.cloudinary.com/" + url.PathEscape(c.Cloud) + "/image/upload/c_fill,g_face,w_320,h_320,q_auto,f_auto/" + deliveryPath(publicID)
}

// CoachingPlaybackURL は非公開の添削動画を再生する署名付き URL を返す。
// du_30 で、万一長い動画が入っていても30秒までしか配信しない。
func (c Client) CoachingPlaybackURL(publicID string) string {
	return c.signedVideoURL("q_auto,du_"+fmt.Sprint(MaxCoachingSeconds), publicID, "mp4")
}

// CoachingThumbnailURL は非公開の添削動画のサムネイルの署名付き URL を返す。
func (c Client) CoachingThumbnailURL(publicID string) string {
	return c.signedVideoURL("so_auto,c_limit,w_800,f_jpg", publicID, "jpg")
}

// signedVideoURL は非公開の動画を配信する URL を返す。
// トークン認証の鍵があれば、期限付きのトークン（__cld_token__）を付ける。公式 SDK と同じく、そのときは署名を付けない。
// 鍵が無ければ署名（s--XXXXXXXX--）を付ける。署名は「変換/public_id.拡張子」に API シークレットを足した SHA-1 を
// URL 用の Base64 にした先頭8文字。バージョン（v1）は署名に含めない。
func (c Client) signedVideoURL(transformation, publicID, ext string) string {
	if c.Cloud == "" || c.Secret == "" || !ValidCoachingPublicID(publicID) {
		return ""
	}
	source := deliveryPath(publicID) + "." + ext
	base := "https://res.cloudinary.com/" + url.PathEscape(c.Cloud) + "/video/" + CoachingVideoType + "/"
	if c.AuthTokenKey != "" {
		token, err := c.authToken(publicID)
		if err != nil {
			return ""
		}
		return base + transformation + "/v1/" + source + "?" + token
	}
	sum := sha1.Sum([]byte(transformation + "/" + source + c.Secret))
	signature := "s--" + base64.URLEncoding.EncodeToString(sum[:])[:8] + "--"
	return base + signature + "/" + transformation + "/v1/" + source
}

// authToken は、その動画の配信（変換やサムネイルを含む）だけに使える、AuthTokenTTL で切れるトークンを返す。
func (c Client) authToken(publicID string) (string, error) {
	key, err := hex.DecodeString(c.AuthTokenKey)
	if err != nil {
		return "", err
	}
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	acl := "/video/" + CoachingVideoType + "/*" + publicID + "*"
	parts := fmt.Sprintf("exp=%d~acl=%s", now().Add(AuthTokenTTL).Unix(), escapeToLower(acl))
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(parts))
	return "__cld_token__=" + parts + "~hmac=" + hex.EncodeToString(mac.Sum(nil)), nil
}

// escapeToLower は、トークンに入れる文字列を公式 SDK と同じ規則（記号を小文字の %xx に）でエスケープする。
func escapeToLower(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch < 0x20 || ch >= 0x7f || strings.IndexByte(" \"#%&'/:;<=>?@[\\]^`{|}~", ch) >= 0 {
			fmt.Fprintf(&b, "%%%02x", ch)
			continue
		}
		b.WriteByte(ch)
	}
	return b.String()
}

func deliveryPath(publicID string) string {
	parts := strings.Split(publicID, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

// Delete は投稿から外れた写真・動画を Cloudinary から消す。見つからないものは消えたものとして扱う。
func (c Client) Delete(ctx context.Context, resource string, publicIDs []string) error {
	if !c.Configured() || len(publicIDs) == 0 {
		return nil
	}
	if resource != "image" && resource != "video" {
		return ErrInvalid
	}
	for _, id := range publicIDs {
		if !ValidPublicID(id) {
			return ErrInvalid
		}
	}
	return c.deleteResources(ctx, resource, "upload", publicIDs)
}

// DeleteCoachingVideos は、非公開の添削・やりとりの動画を Cloudinary から消す。見つからないものは消えたものとして扱う。
func (c Client) DeleteCoachingVideos(ctx context.Context, publicIDs []string) error {
	if !c.Configured() || len(publicIDs) == 0 {
		return nil
	}
	for _, id := range publicIDs {
		if !ValidCoachingPublicID(id) {
			return ErrInvalid
		}
	}
	return c.deleteResources(ctx, "video", CoachingVideoType, publicIDs)
}

func (c Client) deleteResources(ctx context.Context, resource, deliveryType string, publicIDs []string) error {
	values := url.Values{"invalidate": {"true"}}
	for _, id := range publicIDs {
		values.Add("public_ids[]", id)
	}
	endpoint := c.base() + "/v1_1/" + url.PathEscape(c.Cloud) + "/resources/" + resource + "/" + deliveryType + "?" + values.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return ErrUnavailable
	}
	req.SetBasicAuth(c.Key, c.Secret)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return ErrUnavailable
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("%w: delete %s (%d)", ErrUnavailable, resource, resp.StatusCode)
	}
	return nil
}

func (c Client) Ready(ctx context.Context, publicID string) error {
	return c.ready(ctx, "video", publicID)
}

func (c Client) ImageReady(ctx context.Context, publicID string) error {
	return c.ready(ctx, "image", publicID)
}

func (c Client) ready(ctx context.Context, resource, publicID string) error {
	if !ValidPublicID(publicID) {
		return ErrInvalid
	}
	if !c.Configured() {
		return ErrNotConfigured
	}
	endpoint := c.base() + "/v1_1/" + url.PathEscape(c.Cloud) + "/resources/" + resource + "/upload/" + deliveryPath(publicID)
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
	var info struct {
		Bytes        int64  `json:"bytes"`
		Format       string `json:"format"`
		ResourceType string `json:"resource_type"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&info); err != nil {
		return ErrUnavailable
	}
	_, formats, maxBytes, err := presetFor(resource)
	if err != nil {
		return err
	}
	if info.ResourceType != resource || !formatAllowed(info.Format, formats) {
		return ErrRejected
	}
	if info.Bytes <= 0 || info.Bytes > maxBytes {
		return ErrTooLarge
	}
	return nil
}

// CoachingVideoReady は、非公開でアップロードされた添削の動画が、形式・大きさ・長さの条件を満たすか確かめる。
func (c Client) CoachingVideoReady(ctx context.Context, publicID string) error {
	if !ValidCoachingPublicID(publicID) {
		return ErrInvalid
	}
	if !c.Configured() {
		return ErrNotConfigured
	}
	endpoint := c.base() + "/v1_1/" + url.PathEscape(c.Cloud) + "/resources/video/" + CoachingVideoType + "/" + deliveryPath(publicID)
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
	var info struct {
		Bytes        int64   `json:"bytes"`
		Format       string  `json:"format"`
		ResourceType string  `json:"resource_type"`
		Type         string  `json:"type"`
		Duration     float64 `json:"duration"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&info); err != nil {
		return ErrUnavailable
	}
	if info.ResourceType != "video" || info.Type != CoachingVideoType || !formatAllowed(info.Format, VideoFormats) {
		return ErrRejected
	}
	if info.Bytes <= 0 || info.Bytes > MaxVideoBytes {
		return ErrTooLarge
	}
	// 長さが返ってこないときは配信側の du_30 に任せる。
	if info.Duration > MaxCoachingSeconds+coachingSecondsSlop {
		return ErrTooLong
	}
	return nil
}

func formatAllowed(format, allowed string) bool {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "jpeg" {
		format = "jpg"
	}
	if format == "" {
		return false
	}
	for _, item := range strings.Split(allowed, ",") {
		if item == format {
			return true
		}
	}
	return false
}
