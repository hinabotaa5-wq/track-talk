package cloudinary

import (
	"bytes"
	"context"
	"crypto/sha1"
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
)

var (
	ErrNotConfigured = errors.New("cloudinary not configured")
	ErrInvalid       = errors.New("invalid public id")
	ErrNotFound      = errors.New("video not found")
	ErrRejected      = errors.New("asset rejected")
	ErrTooLarge      = errors.New("asset too large")
	ErrUnavailable   = errors.New("cloudinary unavailable")
)

type Client struct {
	Cloud   string
	Key     string
	Secret  string
	APIBase string
	HTTP    *http.Client
}

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

func (c Client) EnsureUploadPresets(ctx context.Context) error {
	if !c.Configured() {
		return nil
	}
	for _, resource := range []string{"image", "video"} {
		preset, formats, _, err := presetFor(resource)
		if err != nil {
			return err
		}
		if err := c.ensurePreset(ctx, preset, formats); err != nil {
			return err
		}
	}
	return nil
}

func (c Client) ensurePreset(ctx context.Context, name, formats string) error {
	values := url.Values{
		"name":            {name},
		"unsigned":        {"false"},
		"allowed_formats": {formats},
		"overwrite":       {"false"},
		"unique_filename": {"false"},
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
	return "https://res.cloudinary.com/" + url.PathEscape(c.Cloud) + "/video/upload/q_auto/" + deliveryPath(publicID) + ".mp4"
}

func (c Client) ImageURL(publicID string) string {
	if c.Cloud == "" || publicID == "" {
		return ""
	}
	return "https://res.cloudinary.com/" + url.PathEscape(c.Cloud) + "/image/upload/c_limit,w_1600,q_auto,f_auto/" + deliveryPath(publicID)
}

func deliveryPath(publicID string) string {
	parts := strings.Split(publicID, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
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
