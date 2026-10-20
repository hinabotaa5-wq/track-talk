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
		grantUpload(t, h, tok, photo)
		rec := doJSON(t, h, http.MethodPost, "/v1/posts", tok, map[string]any{"title": "写真の投稿", "body": "", "image_public_id": photo})
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

// コーチの顔写真の ID を自分の投稿に付けてから消しても、顔写真は消えない。
func TestDeletingAPostKeepsACoachPhoto(t *testing.T) {
	deleted := make(chan []string, 4)
	asset := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodDelete {
			deleted <- r.URL.Query()["public_ids[]"]
			_, _ = w.Write([]byte(`{"deleted":{}}`))
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
	tc := token(t, uuid.New())
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", tc, coachApplicationBody()); rec.Code != http.StatusOK {
		t.Fatalf("apply %d", rec.Code)
	}
	photo := "track-talk/" + uuid.NewString()
	grantUpload(t, h, tc, photo)
	if rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/photo", tc, map[string]any{"photo_public_id": photo}); rec.Code != http.StatusOK {
		t.Fatalf("photo %d %s", rec.Code, rec.Body.String())
	}

	tb := token(t, uuid.New())
	grantUpload(t, h, tb, photo)
	rec := doJSON(t, h, http.MethodPost, "/v1/posts", tb, map[string]any{"title": "写真の投稿", "body": "", "image_public_id": photo})
	if rec.Code != 201 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if rec := doJSON(t, h, http.MethodDelete, "/v1/posts/"+created.ID, tb, nil); rec.Code != 204 {
		t.Fatalf("delete %d", rec.Code)
	}
	select {
	case ids := <-deleted:
		t.Fatalf("coach photo was deleted: %v", ids)
	case <-time.After(300 * time.Millisecond):
	}
}

// 退会したら、その人の投稿の写真も Cloudinary から消す。
func TestWithdrawRemovesPhotosFromCloudinary(t *testing.T) {
	deleted := make(chan []string, 4)
	asset := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodDelete {
			deleted <- r.URL.Query()["public_ids[]"]
			_, _ = w.Write([]byte(`{"deleted":{}}`))
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
	photo := "track-talk/" + uuid.NewString()
	grantUpload(t, h, ta, photo)
	if rec := doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{"title": "写真の投稿", "body": "", "image_public_id": photo}); rec.Code != 201 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodDelete, "/v1/me", ta, map[string]string{"confirm": "退会する"}); rec.Code != http.StatusOK {
		t.Fatalf("withdraw %d %s", rec.Code, rec.Body.String())
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

// 運営が非表示にした投稿を投稿者が消しても、写真は Cloudinary から消える。
func TestDeletingAHiddenPostRemovesItsPhoto(t *testing.T) {
	deleted := make(chan []string, 4)
	asset := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodDelete {
			deleted <- r.URL.Query()["public_ids[]"]
			_, _ = w.Write([]byte(`{"deleted":{}}`))
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
	photo := "track-talk/" + uuid.NewString()
	grantUpload(t, h, ta, photo)
	rec := doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{"title": "写真の投稿", "body": "", "image_public_id": photo})
	if rec.Code != 201 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/posts/"+created.ID+"/hide", adminToken(t, h), nil); rec.Code != http.StatusNoContent {
		t.Fatalf("hide %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodDelete, "/v1/posts/"+created.ID, ta, nil); rec.Code != 204 {
		t.Fatalf("delete %d", rec.Code)
	}
	select {
	case ids := <-deleted:
		if len(ids) != 1 || ids[0] != photo {
			t.Fatalf("deleted %v", ids)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("photo of a hidden post was not deleted")
	}
}

// 大きさの上限を超えて投稿を断った写真は、Cloudinary に残さず消す。
func TestRejectedPhotoIsRemovedFromCloudinary(t *testing.T) {
	deleted := make(chan []string, 4)
	asset := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodDelete {
			deleted <- r.URL.Query()["public_ids[]"]
			_, _ = w.Write([]byte(`{"deleted":{}}`))
			return
		}
		_, _ = w.Write([]byte(`{"resource_type":"image","format":"jpg","bytes":999999999}`))
	}))
	defer asset.Close()
	t.Setenv("CLOUDINARY_CLOUD_NAME", "demo")
	t.Setenv("CLOUDINARY_API_KEY", "key")
	t.Setenv("CLOUDINARY_API_SECRET", "secret")
	t.Setenv("CLOUDINARY_API_BASE", asset.URL)

	h := setup(t)
	photo := "track-talk/" + uuid.NewString()
	tx := token(t, uuid.New())
	grantUpload(t, h, tx, photo)
	rec := doJSON(t, h, http.MethodPost, "/v1/posts", tx, map[string]any{"title": "大きな写真", "body": "", "image_public_id": photo})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	select {
	case ids := <-deleted:
		if len(ids) != 1 || ids[0] != photo {
			t.Fatalf("deleted %v", ids)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("rejected photo was not deleted")
	}
}
