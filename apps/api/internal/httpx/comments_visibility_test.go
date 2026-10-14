package httpx_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestCommentsOfInvisiblePostsAreHidden(t *testing.T) {
	h := setup(t)
	author := token(t, uuid.New())
	adminID := uuid.New()
	admin := token(t, adminID)

	postID := createTextPost(t, h, author, "見える投稿")
	if rec := doJSON(t, h, http.MethodPost, "/v1/posts/"+postID+"/comments", author, map[string]string{"body": "本文"}); rec.Code != http.StatusCreated {
		t.Fatalf("comment %d %s", rec.Code, rec.Body.String())
	}
	if total := commentTotal(t, h, postID); total != 1 {
		t.Fatalf("visible comments %d", total)
	}

	if rec := doJSON(t, h, http.MethodDelete, "/v1/posts/"+postID, author, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/posts/"+postID+"/comments", "", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("deleted comments %d %s", rec.Code, rec.Body.String())
	}

	hiddenID := createTextPost(t, h, author, "隠す投稿")
	if rec := doJSON(t, h, http.MethodPost, "/v1/posts/"+hiddenID+"/comments", author, map[string]string{"body": "隠れる本文"}); rec.Code != http.StatusCreated {
		t.Fatalf("hidden comment %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", admin, nil); rec.Code != http.StatusOK {
		t.Fatalf("admin profile %d %s", rec.Code, rec.Body.String())
	}
	setAdmin(t, adminID)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/posts/"+hiddenID+"/hide", admin, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("hide %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/posts/"+hiddenID+"/comments", "", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("hidden comments %d %s", rec.Code, rec.Body.String())
	}
}

func createTextPost(t *testing.T, h http.Handler, tok, title string) string {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/posts", tok, map[string]any{
		"title": title, "body": "本文です",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	return created.ID
}

func commentTotal(t *testing.T, h http.Handler, postID string) int {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/posts/"+postID+"/comments", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("comments %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return body.Meta.Total
}
