package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// 投稿の作成の冪等性(#382)。

type postAttempt struct {
	code int
	id   string
	body string
}

func postAttemptFrom(rec *httptest.ResponseRecorder) postAttempt {
	var out struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return postAttempt{code: rec.Code, id: out.ID, body: rec.Body.String()}
}

// postConcurrently は同じ本文(と同じトークン)の POST /v1/posts を n 件同時に送る。
func postConcurrently(t *testing.T, h http.Handler, toks []string, bodies []map[string]any) []postAttempt {
	t.Helper()
	out := make([]postAttempt, len(bodies))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range bodies {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			out[i] = postAttemptFrom(doJSON(t, h, http.MethodPost, "/v1/posts", toks[i], bodies[i]))
		}(i)
	}
	close(start)
	wg.Wait()
	return out
}

func repeat[T any](v T, n int) []T {
	out := make([]T, n)
	for i := range out {
		out[i] = v
	}
	return out
}

func photoCloud(t *testing.T) {
	t.Helper()
	asset := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodDelete {
			_, _ = w.Write([]byte(`{"deleted":{}}`))
			return
		}
		_, _ = w.Write([]byte(`{"resource_type":"image","format":"jpg","bytes":1024}`))
	}))
	t.Cleanup(asset.Close)
	t.Setenv("CLOUDINARY_CLOUD_NAME", "demo")
	t.Setenv("CLOUDINARY_API_KEY", "key")
	t.Setenv("CLOUDINARY_API_SECRET", "secret")
	t.Setenv("CLOUDINARY_API_BASE", asset.URL)
}

// 同じ request_key の同時リクエストは、投稿を1件だけ作り、全員がその投稿を受け取る。
func TestCreatePostSameKeyConcurrentCreatesOnePost(t *testing.T) {
	photoCloud(t)
	h := setup(t)
	author := uuid.New()
	tok := token(t, author)
	photo := "track-talk/" + uuid.NewString()
	grantUpload(t, h, tok, photo)
	practiceAt := time.Now().Add(72 * time.Hour).Format(time.RFC3339)

	cases := map[string]map[string]any{
		"text":  {"title": "同時の投稿", "body": "本文"},
		"photo": {"title": "", "body": "", "image_public_id": photo},
		"practice": {"kind": "practice", "body": "一緒に走りましょう", "venue": "競技場",
			"starts_at": practiceAt, "event": "100m", "capacity": 2, "prefecture": "13"},
	}
	for name, base := range cases {
		t.Run(name, func(t *testing.T) {
			key := uuid.NewString()
			body := map[string]any{"request_key": key}
			for k, v := range base {
				body[k] = v
			}
			got := postConcurrently(t, h, repeat(tok, 6), repeat(body, 6))
			created := 0
			for _, g := range got {
				if g.code == http.StatusCreated {
					created++
				} else if g.code != http.StatusOK {
					t.Fatalf("unexpected %d %s", g.code, g.body)
				}
				if g.id == "" || g.id != got[0].id {
					t.Fatalf("different posts returned: %+v", got)
				}
			}
			if created != 1 {
				t.Fatalf("created %d times: %+v", created, got)
			}
			if n := scalar(t, `SELECT count(*) FROM posts WHERE author_id = $1 AND request_key = $2::uuid`, author, key); n != 1 {
				t.Fatalf("posts with key = %d", n)
			}
		})
	}
}

// 同じ request_key を後から送り直すと、1件目をそのまま返す。
func TestCreatePostSameKeySequentialReturnsFirst(t *testing.T) {
	h := setup(t)
	author := uuid.New()
	tok := token(t, author)
	key := uuid.NewString()
	first := postAttemptFrom(doJSON(t, h, http.MethodPost, "/v1/posts", tok, map[string]any{"title": "1回目", "body": "本文", "request_key": key}))
	if first.code != http.StatusCreated {
		t.Fatalf("first %d %s", first.code, first.body)
	}
	second := postAttemptFrom(doJSON(t, h, http.MethodPost, "/v1/posts", tok, map[string]any{"title": "1回目", "body": "本文", "request_key": key}))
	if second.code != http.StatusOK || second.id != first.id {
		t.Fatalf("second %d %s (first %s)", second.code, second.body, first.id)
	}
	if n := scalar(t, `SELECT count(*) FROM posts WHERE author_id = $1`, author); n != 1 {
		t.Fatalf("posts = %d", n)
	}
	// 別の人が同じキーを使っても、その人の投稿として作られる。
	other := postAttemptFrom(doJSON(t, h, http.MethodPost, "/v1/posts", token(t, uuid.New()), map[string]any{"title": "別の人", "body": "本文", "request_key": key}))
	if other.code != http.StatusCreated || other.id == first.id {
		t.Fatalf("other %d %s", other.code, other.body)
	}
	// 消した投稿のキーで送り直しても、作り直さない。
	if rec := doJSON(t, h, http.MethodDelete, "/v1/posts/"+first.id, tok, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete %d", rec.Code)
	}
	again := doJSON(t, h, http.MethodPost, "/v1/posts", tok, map[string]any{"title": "1回目", "body": "本文", "request_key": key})
	if again.Code != http.StatusConflict {
		t.Fatalf("after delete %d %s", again.Code, again.Body.String())
	}
	if n := scalar(t, `SELECT count(*) FROM posts WHERE author_id = $1`, author); n != 1 {
		t.Fatalf("posts after delete = %d", n)
	}
}

// キーを送らない古いクライアントは、従来どおり毎回作る。
func TestCreatePostWithoutKeyKeepsCreating(t *testing.T) {
	h := setup(t)
	author := uuid.New()
	tok := token(t, author)
	for i := 0; i < 2; i++ {
		if rec := doJSON(t, h, http.MethodPost, "/v1/posts", tok, map[string]any{"title": "キーなし", "body": "本文"}); rec.Code != http.StatusCreated {
			t.Fatalf("create %d %s", rec.Code, rec.Body.String())
		}
	}
	if n := scalar(t, `SELECT count(*) FROM posts WHERE author_id = $1`, author); n != 2 {
		t.Fatalf("posts = %d", n)
	}
}

// 同じ写真を別のキーで同時に投稿すると、1件だけ成功し、ほかは項目のエラーになる。
func TestCreatePostSameMediaDifferentKeysOnlyOneSucceeds(t *testing.T) {
	photoCloud(t)
	h := setup(t)
	author := uuid.New()
	tok := token(t, author)
	photo := "track-talk/" + uuid.NewString()
	grantUpload(t, h, tok, photo)
	bodies := make([]map[string]any, 6)
	for i := range bodies {
		bodies[i] = map[string]any{"title": "", "body": "", "image_public_id": photo, "request_key": uuid.NewString()}
	}
	got := postConcurrently(t, h, repeat(tok, len(bodies)), bodies)
	created := 0
	for _, g := range got {
		switch g.code {
		case http.StatusCreated:
			created++
		case http.StatusUnprocessableEntity:
			if !strings.Contains(g.body, "image_public_id") || !strings.Contains(g.body, "この写真はすでに投稿されています") {
				t.Fatalf("error body %s", g.body)
			}
		default:
			t.Fatalf("unexpected %d %s", g.code, g.body)
		}
	}
	if created != 1 {
		t.Fatalf("created %d: %+v", created, got)
	}
	if n := scalar(t, `SELECT count(*) FROM posts WHERE cloudinary_public_id = $1 AND deleted_at IS NULL`, photo); n != 1 {
		t.Fatalf("posts with photo = %d", n)
	}
}
