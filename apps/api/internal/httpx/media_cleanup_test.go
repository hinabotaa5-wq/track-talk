package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestDeletingAPostRemovesItsPhotoFromCloudinary(t *testing.T) {
	deleted := make(chan []string, 4)
	asset := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "key" || pass != "secret" {
			http.Error(w, "auth", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodDelete {
			deleted <- r.URL.Query()["public_ids[]"]
			_, _ = w.Write([]byte(`{"deleted":{}}`))
			return
		}
		if !strings.Contains(r.URL.Path, "/resources/image/upload/track-talk/") {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"resource_type":"image","format":"jpg","bytes":1024}`))
	}))
	defer asset.Close()
	t.Setenv("CLOUDINARY_CLOUD_NAME", "demo")
	t.Setenv("CLOUDINARY_API_KEY", "key")
	t.Setenv("CLOUDINARY_API_SECRET", "secret")
	t.Setenv("CLOUDINARY_API_BASE", asset.URL)

	h := setup(t)
	ta := token(t, uuid.New())
	tb := token(t, uuid.New())
	photo := "track-talk/" + uuid.NewString()
	create := func(tok string) string {
		rec := doJSON(t, h, http.MethodPost, "/v1/posts", tok, map[string]any{"title": "", "body": "", "image_public_id": photo})
		if rec.Code != 201 {
			t.Fatalf("create %d %s", rec.Code, rec.Body.String())
		}
		var created struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &created)
		return created.ID
	}
	first := create(ta)
	// 他人の写真の ID を付けた投稿を消しても、元の投稿の写真は消えない。
	copied := create(tb)
	if rec := doJSON(t, h, http.MethodDelete, "/v1/posts/"+copied, tb, nil); rec.Code != 204 {
		t.Fatalf("delete copied %d", rec.Code)
	}
	select {
	case ids := <-deleted:
		t.Fatalf("photo in use was deleted: %v", ids)
	case <-time.After(300 * time.Millisecond):
	}

	if rec := doJSON(t, h, http.MethodDelete, "/v1/posts/"+first, ta, nil); rec.Code != 204 {
		t.Fatalf("delete %d", rec.Code)
	}
	select {
	case ids := <-deleted:
		if len(ids) != 1 || ids[0] != photo {
			t.Fatalf("deleted %v", ids)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("photo was not deleted")
	}
}
