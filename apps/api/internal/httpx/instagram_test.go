package httpx_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestCiteInstagramOpensOneThreadPerPost(t *testing.T) {
	h := setup(t)
	ta := token(t, uuid.New())

	rec := doJSON(t, h, http.MethodPost, "/v1/posts/instagram", ta, map[string]any{
		"url": "https://www.instagram.com/reel/C1a2B3c4D5e/?igsh=abc", "body": "スタートの反応がすごい"})
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
		"url": "https://instagram.com/someone/reel/C1a2B3c4D5e"})
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
		"url": "https://www.instagram.com/p/D9z8Y7x6W5v/"})
	if rec.Code != 201 {
		t.Fatalf("no title %d %s", rec.Code, rec.Body.String())
	}
	var untitled struct {
		Post struct {
			Title string `json:"title"`
		} `json:"post"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &untitled)
	if untitled.Post.Title != "Instagramの投稿" {
		t.Fatalf("default title %q", untitled.Post.Title)
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
