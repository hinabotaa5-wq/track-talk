package cloudinary

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// StoredAsset は回収前の使用確認に必要な属性だけを持つ。
type StoredAsset struct {
	PublicID  string    `json:"public_id"`
	CreatedAt time.Time `json:"created_at"`
}

// ListStoredAssets はアプリ専用フォルダの1ページだけを取得する。
func (c Client) ListStoredAssets(ctx context.Context, resource, deliveryType, cursor string) ([]StoredAsset, string, error) {
	if !c.Configured() {
		return nil, "", ErrNotConfigured
	}
	prefix := Folder + "/"
	if deliveryType == CoachingVideoType && resource == "video" {
		prefix = CoachingFolder + "/"
	} else if deliveryType != "upload" || (resource != "image" && resource != "video") {
		return nil, "", ErrInvalid
	}
	values := url.Values{"prefix": {prefix}, "max_results": {"500"}}
	if cursor != "" {
		values.Set("next_cursor", cursor)
	}
	endpoint := c.base() + "/v1_1/" + url.PathEscape(c.Cloud) + "/resources/" + resource + "/" + deliveryType + "?" + values.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, "", err
	}
	req.SetBasicAuth(c.Key, c.Secret)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("%w: list assets (%d)", ErrUnavailable, resp.StatusCode)
	}
	var body struct {
		Resources  []StoredAsset `json:"resources"`
		NextCursor string        `json:"next_cursor"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&body); err != nil {
		return nil, "", err
	}
	return body.Resources, body.NextCursor, nil
}

// DeleteStoredAsset は個別の削除結果まで確認する。失敗した分は次の日に再試行される。
func (c Client) DeleteStoredAsset(ctx context.Context, resource, deliveryType, id string) error {
	if !c.Configured() {
		return ErrNotConfigured
	}
	if deliveryType == CoachingVideoType {
		if resource != "video" || !ValidCoachingPublicID(id) {
			return ErrInvalid
		}
	} else if deliveryType != "upload" || (resource != "image" && resource != "video") || !ValidPublicID(id) {
		return ErrInvalid
	}
	values := url.Values{"invalidate": {"true"}, "public_ids[]": {id}}
	endpoint := c.base() + "/v1_1/" + url.PathEscape(c.Cloud) + "/resources/" + resource + "/" + deliveryType + "?" + values.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.Key, c.Secret)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: delete asset (%d)", ErrUnavailable, resp.StatusCode)
	}
	var body struct {
		Deleted map[string]string `json:"deleted"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return err
	}
	if status := body.Deleted[id]; status != "deleted" && status != "not_found" {
		return fmt.Errorf("%w: asset deletion was not confirmed", ErrUnavailable)
	}
	return nil
}
