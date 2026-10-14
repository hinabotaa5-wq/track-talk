package httpx_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestHiddenYouTubeAllowsNewThread(t *testing.T) {
	h := setup(t)
	author := token(t, uuid.New())
	adminID := uuid.New()
	admin := token(t, adminID)
	body := map[string]any{
		"title": "公式", "body": "", "kind": "youtube",
		"youtube_video_id": "abcdefghijk", "youtube_thumbnail_url": "https://i.ytimg.com/vi/abcdefghijk/hqdefault.jpg",
	}

	rec := doJSON(t, h, http.MethodPost, "/v1/posts", author, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var first struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &first)

	if rec := doJSON(t, h, http.MethodGet, "/v1/me", admin, nil); rec.Code != http.StatusOK {
		t.Fatalf("admin profile %d %s", rec.Code, rec.Body.String())
	}
	setAdmin(t, adminID)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/posts/"+first.ID+"/hide", admin, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("hide %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts", author, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("recreate %d %s", rec.Code, rec.Body.String())
	}
	var second struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &second)
	if second.ID == "" || second.ID == first.ID {
		t.Fatalf("second id %q first %q", second.ID, first.ID)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts/youtube", author, map[string]any{
		"url": "https://youtu.be/abcdefghijk",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("cite %d %s", rec.Code, rec.Body.String())
	}
	var cited struct {
		Existing bool `json:"existing"`
		Post     struct {
			ID string `json:"id"`
		} `json:"post"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &cited)
	if !cited.Existing || cited.Post.ID != second.ID {
		t.Fatalf("cited %+v want %s", cited, second.ID)
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/posts/"+first.ID, "", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("hidden post %d", rec.Code)
	}
}
