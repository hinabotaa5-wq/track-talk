package cloudinary

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const Folder = "track-talk"

var (
	ErrNotConfigured = errors.New("cloudinary not configured")
	ErrInvalid       = errors.New("invalid public id")
	ErrNotFound      = errors.New("video not found")
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
	CloudName string `json:"cloud_name"`
	APIKey    string `json:"api_key"`
	Timestamp int64  `json:"timestamp"`
	PublicID  string `json:"public_id"`
	Signature string `json:"signature"`
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

func (c Client) SignedUpload(now time.Time) (UploadSignature, error) {
	if !c.Configured() {
		return UploadSignature{}, ErrNotConfigured
	}
	publicID := NewPublicID()
	ts := now.Unix()
	sig := Sign(map[string]string{
		"public_id": publicID,
		"timestamp": fmt.Sprintf("%d", ts),
	}, c.Secret)
	return UploadSignature{
		CloudName: c.Cloud,
		APIKey:    c.Key,
		Timestamp: ts,
		PublicID:  publicID,
		Signature: sig,
	}, nil
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
	return nil
}
