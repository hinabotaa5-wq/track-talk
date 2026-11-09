package cloudinary

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeOriginals は Admin API の詳細と Upload API の置き換え・印外しを真似る。
type fakeOriginals struct {
	mu      sync.Mutex
	details map[string]string // パス末尾（type/public_id）→ JSON
	posts   []url.Values
	paths   []string
	status  int
}

func (f *fakeOriginals) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.Method {
		case http.MethodGet:
			if user, pass, ok := r.BasicAuth(); !ok || user != "key" || pass != "secret" {
				http.Error(w, "auth", http.StatusUnauthorized)
				return
			}
			if strings.Contains(r.URL.Path, "/resources/video/tags/") {
				if r.URL.Query().Get("next_cursor") == "" {
					fmt.Fprint(w, `{"resources":[{"public_id":"a","type":"upload"}],"next_cursor":"c2"}`)
				} else {
					fmt.Fprint(w, `{"resources":[{"public_id":"b","type":"authenticated"}]}`)
				}
				return
			}
			for suffix, body := range f.details {
				if strings.HasSuffix(r.URL.Path, suffix) {
					fmt.Fprint(w, body)
					return
				}
			}
			http.NotFound(w, r)
		case http.MethodPost:
			if _, _, ok := r.BasicAuth(); ok {
				t.Errorf("upload API must not get basic auth")
			}
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			f.posts = append(f.posts, r.PostForm)
			f.paths = append(f.paths, r.URL.Path)
			if f.status != 0 {
				http.Error(w, "boom", f.status)
				return
			}
			fmt.Fprint(w, `{}`)
		}
	}
}

func signedParamsOK(t *testing.T, v url.Values) {
	t.Helper()
	params := map[string]string{}
	for k := range v {
		if k == "signature" || k == "api_key" || k == "file" {
			continue
		}
		params[k] = v.Get(k)
	}
	if got, want := v.Get("signature"), Sign(params, "secret"); got != want {
		t.Fatalf("signature %s want %s", got, want)
	}
	if v.Get("api_key") != "key" {
		t.Fatalf("api_key %q", v.Get("api_key"))
	}
}

func newOriginalsClient(f *fakeOriginals, t *testing.T) (Client, func()) {
	srv := httptest.NewServer(f.handler(t))
	return Client{Cloud: "demo", Key: "key", Secret: "secret", APIBase: srv.URL, Now: func() time.Time { return time.Unix(1700000000, 0) }}, srv.Close
}

func TestShrinkOriginalReplacesPublicVideoWithPlaybackDerived(t *testing.T) {
	id := NewPublicID()
	f := &fakeOriginals{details: map[string]string{
		"/upload/" + id: `{"tags":["tt-original"],"derived":[{"transformation":"c_limit,h_1280,q_auto,w_1280/mp4","format":"mp4"}]}`,
	}}
	c, done := newOriginalsClient(f, t)
	defer done()
	if err := c.ShrinkOriginal(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if len(f.posts) != 2 || !strings.HasSuffix(f.paths[0], "/v1_1/demo/video/upload") || !strings.HasSuffix(f.paths[1], "/v1_1/demo/video/tags") {
		t.Fatalf("paths %v", f.paths)
	}
	up := f.posts[0]
	signedParamsOK(t, up)
	if up.Get("file") != c.PlaybackURL(id) || up.Get("public_id") != id || up.Get("type") != "upload" ||
		up.Get("overwrite") != "true" || up.Get("invalidate") != "true" ||
		up.Get("eager") != eagerFor(publicVideoDerived) || up.Get("eager_async") != "true" || up.Get("timestamp") != "1700000000" {
		t.Fatalf("upload form %v", up)
	}
	if up.Get("upload_preset") != "" || up.Get("tags") != "" {
		t.Fatalf("replacement must not use a preset or tags: %v", up)
	}
	tags := f.posts[1]
	signedParamsOK(t, tags)
	if tags.Get("command") != "remove" || tags.Get("tag") != OriginalTag || tags.Get("public_ids") != id || tags.Get("type") != "upload" {
		t.Fatalf("tags form %v", tags)
	}
}

func TestShrinkOriginalReplacesCoachingVideoFromSignedURL(t *testing.T) {
	id := NewCoachingPublicID()
	f := &fakeOriginals{details: map[string]string{
		"/authenticated/" + id: `{"tags":["tt-original"],"derived":[{"transformation":"c_limit,w_1280,h_1280,q_auto,du_30/mp4","format":"mp4"}]}`,
	}}
	c, done := newOriginalsClient(f, t)
	defer done()
	if err := c.ShrinkOriginal(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	up := f.posts[0]
	if up.Get("type") != CoachingVideoType || up.Get("file") != c.CoachingPlaybackURL(id) || up.Get("file") == "" ||
		up.Get("eager") != eagerFor(coachingVideoDerived) {
		t.Fatalf("upload form %v", up)
	}
	if f.posts[1].Get("type") != CoachingVideoType {
		t.Fatalf("tags form %v", f.posts[1])
	}
}

func TestShrinkOriginalWaitsAndSkips(t *testing.T) {
	id := NewPublicID()
	f := &fakeOriginals{details: map[string]string{}}
	c, done := newOriginalsClient(f, t)
	defer done()
	ctx := context.Background()
	// 再生用の派生がまだ（サムネイルだけ）なら待つ。
	f.details["/upload/"+id] = `{"tags":["tt-original"],"derived":[{"transformation":"so_auto,c_limit,w_800,q_auto,f_jpg/jpg","format":"jpg"}]}`
	if err := c.ShrinkOriginal(ctx, id); !errors.Is(err, ErrProcessing) {
		t.Fatalf("processing: %v", err)
	}
	// 印が無い（置き換え済み・古い画面で縮めて上げた）なら何もしない。
	f.details["/upload/"+id] = `{"tags":[],"derived":[{"transformation":"c_limit,h_1280,q_auto,w_1280/mp4","format":"mp4"}]}`
	if err := c.ShrinkOriginal(ctx, id); !errors.Is(err, ErrNotOriginal) {
		t.Fatalf("not original: %v", err)
	}
	if err := c.ShrinkOriginal(ctx, NewPublicID()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	if err := c.ShrinkOriginal(ctx, "other/"+id); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid: %v", err)
	}
	if len(f.posts) != 0 {
		t.Fatalf("must not upload: %v", f.posts)
	}
	if err := (Client{}).ShrinkOriginal(ctx, id); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("not configured: %v", err)
	}
}

func TestShrinkOriginalReportsUploadFailure(t *testing.T) {
	id := NewPublicID()
	f := &fakeOriginals{status: http.StatusBadRequest, details: map[string]string{
		"/upload/" + id: `{"tags":["tt-original"],"derived":[{"transformation":"c_limit,h_1280,q_auto,w_1280/mp4","format":"mp4"}]}`,
	}}
	c, done := newOriginalsClient(f, t)
	defer done()
	if err := c.ShrinkOriginal(context.Background(), id); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err %v", err)
	}
	if len(f.posts) != 1 {
		t.Fatalf("tag must stay when the upload failed: %v", f.paths)
	}
}

func TestListOriginalsPages(t *testing.T) {
	f := &fakeOriginals{}
	c, done := newOriginalsClient(f, t)
	defer done()
	page, next, err := c.ListOriginals(context.Background(), "")
	if err != nil || len(page) != 1 || page[0].PublicID != "a" || next != "c2" {
		t.Fatalf("page1 %v %q %v", page, next, err)
	}
	page, next, err = c.ListOriginals(context.Background(), next)
	if err != nil || len(page) != 1 || page[0].Type != "authenticated" || next != "" {
		t.Fatalf("page2 %v %q %v", page, next, err)
	}
}
