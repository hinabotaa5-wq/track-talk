package httpx_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestCiteXPostDeduplicatesAndAppearsInFeeds(t *testing.T) {
	h := setup(t)
	user := token(t, uuid.New())
	var original string
	for i, raw := range []string{"https://x.com/runner/status/123456789?s=46", "https://twitter.com/runner/status/123456789/photo/1"} {
		rec := doJSON(t, h, http.MethodPost, "/v1/posts/x", user, map[string]any{"url": raw, "body": "参考にしたい走り"})
		expected := http.StatusCreated
		if i > 0 {
			expected = http.StatusOK
		}
		if rec.Code != expected {
			t.Fatalf("citation %d: %d %s", i, rec.Code, rec.Body.String())
		}
		var got struct {
			Existing bool `json:"existing"`
			Post     struct {
				ID   string `json:"id"`
				Kind string `json:"kind"`
				XID  string `json:"x_post_id"`
			} `json:"post"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Post.Kind != "x" || got.Post.XID != "123456789" || got.Existing != (i > 0) {
			t.Fatalf("bad citation: %+v", got)
		}
		if i == 0 {
			original = got.Post.ID
		} else if got.Post.ID != original {
			t.Fatal("duplicate thread")
		}
	}
	for _, path := range []string{"/v1/posts?sort=new&pagination=cursor", "/v1/me/posts?pagination=cursor", "/v1/posts/" + original} {
		rec := doJSON(t, h, http.MethodGet, path, user, nil)
		if rec.Code != 200 || !json.Valid(rec.Body.Bytes()) {
			t.Fatalf("read %s: %d %s", path, rec.Code, rec.Body.String())
		}
		var value map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &value)
		if path == "/v1/posts/"+original {
			if value["x_post_id"] != "123456789" {
				t.Fatal(value)
			}
		} else {
			posts := value["posts"].([]any)
			if len(posts) != 1 || posts[0].(map[string]any)["x_post_id"] != "123456789" {
				t.Fatal(value)
			}
		}
	}
}
func TestCiteXPostRejectsInvalidURLs(t *testing.T) {
	h := setup(t)
	user := token(t, uuid.New())
	for _, raw := range []string{"https://x.com/runner", "https://x.com.evil.test/runner/status/123", "https://x.com/runner/status/nope", "https://u@x.com/runner/status/123"} {
		rec := doJSON(t, h, http.MethodPost, "/v1/posts/x", user, map[string]any{"url": raw})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("accepted %s: %d", raw, rec.Code)
		}
	}
}
