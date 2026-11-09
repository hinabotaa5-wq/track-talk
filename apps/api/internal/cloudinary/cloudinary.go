package cloudinary

import (
	"bytes"
	"context"
	"crypto/hmac"
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
	Folder      = "track-talk"
	ImagePreset = "track-talk-image"
	VideoPreset = "track-talk-video"
	// VideoAsyncPreset は、保存前に縮めない（incoming 変換なし）動画のプリセット。
	// 配信に使う変換は、アップロードの署名に付ける eager（eager_async=true）で裏で作る。
	// Cloudinary は同期で処理できる動画の大きさに上限があり（Free は 40MB）、incoming 変換はその上限に当たるため。
	// https://cloudinary.com/documentation/video_trimming_and_concatenating#troubleshooting_video_transformation_errors
	VideoAsyncPreset = "track-talk-video-async"
	ImageFormats     = "gif,heic,heif,jpg,png,webp"
	VideoFormats     = "mov,mp4,webm"
	MaxImageBytes    = 10 << 20
	MaxVideoBytes    = 100 << 20

	// 添削の動画は顔や体が写るので、公開 URL では配信しない（type=authenticated）。
	// 署名付きの URL を知っている人だけが見られる。
	CoachingFolder      = "track-talk-coaching"
	CoachingVideoType   = "authenticated"
	MaxCoachingSeconds  = 30
	coachingSecondsSlop = 0.5

	// 保存する前に Cloudinary 側で縮める。元ファイルは残らない。
	// 写真は表示(長辺1600px)より少し大きい2048pxまで、動画は720p相当(長辺1280px)まで。
	// 動画の incoming は、署名に eager を付けない古い画面（VideoPreset）のときだけ使う。
	ImageIncoming = "c_limit,w_2048,h_2048"
	VideoIncoming = "c_limit,w_1280,h_1280,q_auto"

	// 動画の配信に使う変換。eager で事前に作る変換と、配信 URL の変換は一字一句同じでなければならない
	// （違うと配信のときに同期で作ろうとして、大きな動画では失敗する）。
	videoPlayback       = "c_limit,w_1280,h_1280,q_auto"
	videoThumbnail      = "so_auto,c_limit,w_800,q_auto,f_jpg"
	videoCardThumbnail  = "so_auto,c_limit,w_" + cardWidth + ",q_auto,f_jpg"
	coachingPlayback    = "c_limit,w_1280,h_1280,q_auto,du_30"
	coachingThumbnail   = "so_auto,c_limit,w_800,f_jpg"
	playbackExt         = "mp4"
	thumbnailExt        = "jpg"
	eagerAsyncParameter = "true"

	// SyncVideoMaxBytes より大きい動画は、eager の派生ができるまで投稿に使わせない。
	// それ以下なら、派生がまだでも配信のときに同期で作れる。Cloudinary の同期処理の上限（Free は 40MB）より
	// 余裕をもって小さくしてある。
	SyncVideoMaxBytes = 30 << 20
)

type derivedSpec struct {
	transformation string
	ext            string
}

func (d derivedSpec) eager() string { return d.transformation + "/" + d.ext }

// publicVideoDerived は公開動画の配信（再生・詳細のサムネイル・カードのサムネイル）。
var publicVideoDerived = []derivedSpec{
	{videoPlayback, playbackExt},
	{videoThumbnail, thumbnailExt},
	{videoCardThumbnail, thumbnailExt},
}

// coachingVideoDerived は非公開の添削・やりとりの動画の配信（再生・サムネイル）。
var coachingVideoDerived = []derivedSpec{
	{coachingPlayback, playbackExt},
	{coachingThumbnail, thumbnailExt},
}

func eagerFor(specs []derivedSpec) string {
	parts := make([]string, len(specs))
	for i, d := range specs {
		parts[i] = d.eager()
	}
	return strings.Join(parts, "|")
}

var (
	ErrNotConfigured = errors.New("cloudinary not configured")
	ErrInvalid       = errors.New("invalid public id")
	ErrNotFound      = errors.New("video not found")
	ErrRejected      = errors.New("asset rejected")
	ErrTooLarge      = errors.New("asset too large")
	ErrTooLong       = errors.New("video too long")
	ErrUnavailable   = errors.New("cloudinary unavailable")
	// ErrProcessing は、大きな動画の配信用の変換を Cloudinary がまだ作っている途中であること。
	ErrProcessing = errors.New("video still processing")
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
	// Eager と EagerAsync は、配信用の変換をアップロード後に裏で作らせる指定。署名に含まれるので、そのまま送る。
	Eager      string `json:"eager,omitempty"`
	EagerAsync string `json:"eager_async,omitempty"`
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
	// Cloudinary は API 署名に SHA-256 を受け付ける。
	// https://cloudinary.com/documentation/authentication_signatures
	sum := sha256.Sum256([]byte(strings.Join(parts, "&") + secret))
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
	return c.signedUpload(now, resource, false)
}

// SignedAsyncVideoUpload は、保存前に縮めず、配信用の変換を eager（非同期）で作らせる公開動画の署名を返す。
// eager と eager_async を送れる画面だけが使う（送らないと署名が合わず、Cloudinary が断る）。
func (c Client) SignedAsyncVideoUpload(now time.Time) (UploadSignature, error) {
	return c.signedUpload(now, "video", true)
}

func (c Client) signedUpload(now time.Time, resource string, async bool) (UploadSignature, error) {
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
	signed := UploadSignature{
		CloudName:      c.Cloud,
		APIKey:         c.Key,
		Timestamp:      ts,
		PublicID:       publicID,
		UploadPreset:   preset,
		AllowedFormats: formats,
		ResourceType:   resource,
	}
	if async && resource == "video" {
		signed.UploadPreset = VideoAsyncPreset
		signed.Eager = eagerFor(publicVideoDerived)
		signed.EagerAsync = eagerAsyncParameter
		params["upload_preset"] = signed.UploadPreset
		params["eager"] = signed.Eager
		params["eager_async"] = signed.EagerAsync
	}
	signed.Signature = Sign(params, c.Secret)
	return signed, nil
}

// SignedCoachingUpload は添削の動画を非公開（authenticated）でアップロードする署名を返す。
// type も署名に含めるので、ブラウザ側で公開に書き換えると Cloudinary が受け付けない。
func (c Client) SignedCoachingUpload(now time.Time) (UploadSignature, error) {
	return c.signedCoachingUpload(now, false)
}

// SignedAsyncCoachingUpload は、SignedCoachingUpload の、保存前に縮めず eager（非同期）で配信用の変換を作らせる版。
func (c Client) SignedAsyncCoachingUpload(now time.Time) (UploadSignature, error) {
	return c.signedCoachingUpload(now, true)
}

func (c Client) signedCoachingUpload(now time.Time, async bool) (UploadSignature, error) {
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
	signed := UploadSignature{
		CloudName:      c.Cloud,
		APIKey:         c.Key,
		Timestamp:      ts,
		PublicID:       publicID,
		UploadPreset:   VideoPreset,
		AllowedFormats: VideoFormats,
		ResourceType:   "video",
		Type:           CoachingVideoType,
	}
	if async {
		signed.UploadPreset = VideoAsyncPreset
		signed.Eager = eagerFor(coachingVideoDerived)
		signed.EagerAsync = eagerAsyncParameter
		params["upload_preset"] = signed.UploadPreset
		params["eager"] = signed.Eager
		params["eager_async"] = signed.EagerAsync
	}
	signed.Signature = Sign(params, c.Secret)
	return signed, nil
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
	// 縮めない動画のプリセット。配信用の変換は署名の eager で作る。
	return c.ensurePreset(ctx, VideoAsyncPreset, VideoFormats, "")
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
	}
	if transformation != "" {
		values.Set("transformation", transformation)
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
	return "https://res.cloudinary.com/" + url.PathEscape(c.Cloud) + "/video/upload/" + videoThumbnail + "/" + deliveryPath(publicID) + "." + thumbnailExt
}

// CardThumbnailURL は一覧カード用の小さな動画サムネイル URL を返す。
// カードは幅200px前後なので、高解像度の端末でも粗く見えない幅にとどめる。
func (c Client) CardThumbnailURL(publicID string) string {
	if c.Cloud == "" || publicID == "" {
		return ""
	}
	return "https://res.cloudinary.com/" + url.PathEscape(c.Cloud) + "/video/upload/" + videoCardThumbnail + "/" + deliveryPath(publicID) + "." + thumbnailExt
}

func (c Client) PlaybackURL(publicID string) string {
	if c.Cloud == "" || publicID == "" {
		return ""
	}
	return "https://res.cloudinary.com/" + url.PathEscape(c.Cloud) + "/video/upload/" + videoPlayback + "/" + deliveryPath(publicID) + "." + playbackExt
}

func (c Client) ImageURL(publicID string) string {
	if c.Cloud == "" || publicID == "" {
		return ""
	}
	return "https://res.cloudinary.com/" + url.PathEscape(c.Cloud) + "/image/upload/c_limit,w_1600,q_auto,f_auto/" + deliveryPath(publicID)
}

// cardWidth は一覧カードに配信する画像の幅。
const cardWidth = "600"

// CardImageURL は一覧カード用の小さな画像 URL を返す。詳細画面は ImageURL を使う。
func (c Client) CardImageURL(publicID string) string {
	if c.Cloud == "" || publicID == "" {
		return ""
	}
	return "https://res.cloudinary.com/" + url.PathEscape(c.Cloud) + "/image/upload/c_limit,w_" + cardWidth + ",q_auto,f_auto/" + deliveryPath(publicID)
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
// 保存前に縮めなくなった動画も、これまでどおり長辺1280px以内で配信する。
func (c Client) CoachingPlaybackURL(publicID string) string {
	return c.signedVideoURL(coachingPlayback, publicID, playbackExt)
}

// CoachingThumbnailURL は非公開の添削動画のサムネイルの署名付き URL を返す。
func (c Client) CoachingThumbnailURL(publicID string) string {
	return c.signedVideoURL(coachingThumbnail, publicID, thumbnailExt)
}

// signedVideoURL は非公開の動画を配信する URL を返す。
// トークン認証の鍵があれば、期限付きのトークン（__cld_token__）を付ける。公式 SDK と同じく、そのときは署名を付けない。
// 鍵が無ければ署名（s--XXXXXXXX--）を付ける。署名は「変換/public_id.拡張子」に API シークレットを足した SHA-256 を
// URL 用の Base64 にした先頭8文字。バージョン（v1）は署名に含めない。
// https://cloudinary.com/documentation/delivery_url_signatures
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
	sum := sha256.Sum256([]byte(transformation + "/" + source + c.Secret))
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
		Bytes        int64     `json:"bytes"`
		Format       string    `json:"format"`
		ResourceType string    `json:"resource_type"`
		Derived      []derived `json:"derived"`
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
	if resource == "video" && !derivedReady(info.Bytes, info.Derived, publicVideoDerived) {
		return ErrProcessing
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
		Bytes        int64     `json:"bytes"`
		Format       string    `json:"format"`
		ResourceType string    `json:"resource_type"`
		Type         string    `json:"type"`
		Duration     float64   `json:"duration"`
		Derived      []derived `json:"derived"`
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
	if !derivedReady(info.Bytes, info.Derived, coachingVideoDerived) {
		return ErrProcessing
	}
	return nil
}

// derived は、Admin API の資産の詳細に入っている、作り終えた派生の1件。
type derived struct {
	Transformation string `json:"transformation"`
	Format         string `json:"format"`
}

// derivedReady は、配信のときに同期で作れない大きさの動画について、配信に使う派生がすべて作り終わっているかを返す。
// 小さい動画は、派生がまだでも配信のときに作れるので待たない（署名に eager を付けない古い画面の動画もここを通る）。
func derivedReady(bytes int64, have []derived, want []derivedSpec) bool {
	if bytes <= SyncVideoMaxBytes {
		return true
	}
	for _, w := range want {
		found := false
		for _, d := range have {
			if sameDerived(d, w) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// sameDerived は、派生の変換と形式が、配信に使う変換と同じかを返す。
// Cloudinary は変換を "c_limit,w_1280/mp4" のように拡張子付きで返すことがあるので、拡張子は format と合わせて見る。
// 変換の中の項目の並びは比べない。
func sameDerived(d derived, w derivedSpec) bool {
	transformation, ext := d.Transformation, strings.ToLower(d.Format)
	if i := strings.LastIndex(transformation, "/"); i >= 0 {
		if suffix := strings.ToLower(transformation[i+1:]); suffix != "" && !strings.Contains(suffix, "_") && !strings.Contains(suffix, ",") {
			transformation = transformation[:i]
			if ext == "" {
				ext = suffix
			}
		}
	}
	if ext == "jpeg" {
		ext = "jpg"
	}
	if ext != "" && ext != w.ext {
		return false
	}
	return sameComponents(transformation, w.transformation)
}

func sameComponents(a, b string) bool {
	as, bs := strings.Split(a, ","), strings.Split(b, ",")
	if len(as) != len(bs) {
		return false
	}
	sort.Strings(as)
	sort.Strings(bs)
	for i := range as {
		if as[i] != bs[i] {
			return false
		}
	}
	return true
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
