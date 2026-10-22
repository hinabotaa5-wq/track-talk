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

func TestCiteInstagramOpensOneThreadPerPost(t *testing.T) {
	h := setup(t)
	ta := token(t, uuid.New())

	rec := doJSON(t, h, http.MethodPost, "/v1/posts/instagram", ta, map[string]any{
		"url": "https://www.instagram.com/reel/C1a2B3c4D5e/?igsh=abc", "title": "スタートの反応がすごい", "body": "スタートの反応がすごい"})
	if rec.Code != 201 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Existing bool `json:"existing"`
		Post     struct {
			ID                 string `json:"id"`
			Title              string `json:"title"`
			Kind               string `json:"kind"`
			InstagramType      string `json:"instagram_type"`
			InstagramShortcode string `json:"instagram_shortcode"`
			YouTubeVideoID     string `json:"youtube_video_id"`
		} `json:"post"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	p := created.Post
	if created.Existing || p.Kind != "instagram" || p.InstagramType != "reel" || p.InstagramShortcode != "C1a2B3c4D5e" || p.YouTubeVideoID != "" {
		t.Fatalf("created %+v", created)
	}
	if p.Title != "スタートの反応がすごい" {
		t.Fatalf("title %q", p.Title)
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/posts/"+p.ID, ta, nil)
	if rec.Code != 200 {
		t.Fatalf("get %d %s", rec.Code, rec.Body.String())
	}
	var got struct {
		InstagramShortcode string `json:"instagram_shortcode"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.InstagramShortcode != "C1a2B3c4D5e" {
		t.Fatalf("detail %s", rec.Body.String())
	}

	// 同じ投稿を別の形の URL で貼っても、最初のスレッドに集まる
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/instagram", token(t, uuid.New()), map[string]any{
		"url": "https://instagram.com/someone/reel/C1a2B3c4D5e", "title": "別の人のタイトル"})
	if rec.Code != 200 {
		t.Fatalf("cite again %d %s", rec.Code, rec.Body.String())
	}
	var again struct {
		Existing bool `json:"existing"`
		Post     struct {
			ID string `json:"id"`
		} `json:"post"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &again)
	if !again.Existing || again.Post.ID != p.ID {
		t.Fatalf("again %+v want %s", again, p.ID)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts/instagram", ta, map[string]any{
		"url": "https://www.instagram.com/p/D9z8Y7x6W5v/", "title": "別の投稿"})
	if rec.Code != 201 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}

	for _, raw := range []string{
		"https://example.com/p/C1a2B3c4D5e/",
		"https://www.instagram.com.evil.com/p/C1a2B3c4D5e/",
		"https://www.instagram.com/someone/",
	} {
		rec = doJSON(t, h, http.MethodPost, "/v1/posts/instagram", ta, map[string]any{"url": raw})
		if rec.Code != 422 {
			t.Fatalf("%s %d %s", raw, rec.Code, rec.Body.String())
		}
	}

	// 汎用の投稿 API からは Instagram の投稿を作れない
	rec = doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{"kind": "instagram", "title": "x", "body": ""})
	if rec.Code != 422 {
		t.Fatalf("generic create %d %s", rec.Code, rec.Body.String())
	}

	// 編集ではタイトルと本文だけ変わり、埋め込み先は変わらない
	rec = doJSON(t, h, http.MethodPatch, "/v1/posts/"+p.ID, ta, map[string]any{"title": "直した", "body": "本文"})
	if rec.Code != 200 {
		t.Fatalf("edit %d %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.InstagramShortcode != "C1a2B3c4D5e" {
		t.Fatalf("edit lost embed %s", rec.Body.String())
	}
}

// YouTube・Instagram・文章の投稿は、タイトル欄なしで作成・編集できる。
func TestUrlPostsDoNotNeedATitle(t *testing.T) {
	h := setup(t)
	ta := token(t, uuid.New())

	rec := doJSON(t, h, http.MethodPost, "/v1/posts/instagram", ta, map[string]any{
		"url": "https://www.instagram.com/p/E1a2B3c4D5e/", "body": "タイトルなし"})
	if rec.Code != 201 {
		t.Fatalf("create without title %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Post struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"post"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.Post.Title != "" {
		t.Fatalf("title %q", created.Post.Title)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts/instagram", ta, map[string]any{
		"url": "https://www.instagram.com/p/F1a2B3c4D5e/", "title": strings.Repeat("あ", 121)})
	if rec.Code != 422 {
		t.Fatalf("too long title %d", rec.Code)
	}

	rec = doJSON(t, h, http.MethodPatch, "/v1/posts/"+created.Post.ID, ta, map[string]any{"title": "", "body": "タイトルなしのまま"})
	if rec.Code != 200 {
		t.Fatalf("edit without title %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{"kind": "text", "title": "", "body": "本文だけ"})
	if rec.Code != 201 {
		t.Fatalf("text post without title %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{"kind": "text", "title": "", "body": ""})
	if rec.Code != 422 {
		t.Fatalf("empty text post %d %s", rec.Code, rec.Body.String())
	}
}

// 写真のない文章の投稿は、一覧に本文の冒頭（excerpt）が付く。
func TestTextPostsInTheFeedCarryAnExcerpt(t *testing.T) {
	h := setup(t)
	ta := token(t, uuid.New())
	body := "決勝は向かい風だった。\n\n" + strings.Repeat("あ", 200)
	if rec := doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{"kind": "text", "title": "全国大会", "body": body}); rec.Code != 201 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	rec := doJSON(t, h, http.MethodGet, "/v1/posts?sort=new", "", nil)
	var list struct {
		Posts []struct {
			Title   string `json:"title"`
			Body    string `json:"body"`
			Excerpt string `json:"excerpt"`
		} `json:"posts"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list.Posts) == 0 || list.Posts[0].Body != "" || !strings.HasPrefix(list.Posts[0].Excerpt, "決勝は向かい風だった。 ああ") || len([]rune(list.Posts[0].Excerpt)) > 120 {
		t.Fatalf("feed = %s", rec.Body.String())
	}
}

func TestUrlPostsInTheFeedCarryBodyExcerpt(t *testing.T) {
	h := setup(t)
	ta := token(t, uuid.New())
	body := "リールの内容をメモしました。"
	rec := doJSON(t, h, http.MethodPost, "/v1/posts/instagram", ta, map[string]any{
		"url": "https://www.instagram.com/reel/H1a2B3c4D5e/", "body": body,
	})
	if rec.Code != 201 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/posts?sort=new", "", nil)
	var list struct {
		Posts []struct {
			Kind    string `json:"kind"`
			Title   string `json:"title"`
			Excerpt string `json:"excerpt"`
		} `json:"posts"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	for _, post := range list.Posts {
		if post.Kind == "instagram" {
			if post.Title != "" || post.Excerpt != body {
				t.Fatalf("Instagram feed card title=%q excerpt=%q", post.Title, post.Excerpt)
			}
			return
		}
	}
	t.Fatalf("Instagram post not in feed: %s", rec.Body.String())
}

// Instagram の表紙は外から確実に取れないので、投稿者が任意で付けた画像をカードに出す。投稿を消すと画像も消す。
func TestInstagramPostWithCoverImage(t *testing.T) {
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
	cover := "track-talk/" + uuid.NewString()

	// 自分に発行されていない画像は付けられない。
	rec := doJSON(t, h, http.MethodPost, "/v1/posts/instagram", ta, map[string]any{
		"url": "https://www.instagram.com/p/G1a2B3c4D5e/", "image_public_id": cover})
	if rec.Code < 400 {
		t.Fatalf("cover not issued to me = %d %s", rec.Code, rec.Body.String())
	}

	grantUpload(t, h, ta, cover)
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/instagram", ta, map[string]any{
		"url": "https://www.instagram.com/p/G1a2B3c4D5e/", "image_public_id": cover})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Post struct {
			ID           string `json:"id"`
			ThumbnailURL string `json:"thumbnail_url"`
		} `json:"post"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if !strings.Contains(created.Post.ThumbnailURL, cover) {
		t.Fatalf("thumbnail_url = %q", created.Post.ThumbnailURL)
	}
	// フィードのカードにも出る。
	rec = doJSON(t, h, http.MethodGet, "/v1/posts?sort=new", "", nil)
	if !strings.Contains(rec.Body.String(), cover) {
		t.Fatalf("feed has no cover: %s", rec.Body.String())
	}

	if rec := doJSON(t, h, http.MethodDelete, "/v1/posts/"+created.Post.ID, ta, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete %d", rec.Code)
	}
	select {
	case ids := <-deleted:
		if len(ids) != 1 || ids[0] != cover {
			t.Fatalf("deleted %v", ids)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cover was not deleted")
	}
}
