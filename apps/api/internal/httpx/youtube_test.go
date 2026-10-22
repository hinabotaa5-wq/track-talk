package httpx

import (
	"strings"
	"testing"
)

func TestYouTubeIDFromURL(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"https://www.youtube.com/watch?v=abcdefghijk", "abcdefghijk"},
		{"https://m.youtube.com/watch?v=abcdefghijk&t=12s", "abcdefghijk"},
		{"https://youtu.be/abcdefghijk", "abcdefghijk"},
		{"https://www.youtube.com/shorts/abcdefghijk", "abcdefghijk"},
		{"https://www.youtube.com/embed/abcdefghijk", "abcdefghijk"},
		{"https://music.youtube.com/watch?v=abcdefghijk", "abcdefghijk"},
	}
	for _, tc := range cases {
		got, err := YouTubeIDFromURL(tc.raw)
		if err != "" || got != tc.want {
			t.Fatalf("%s -> %q %q", tc.raw, got, err)
		}
	}
	if _, err := YouTubeIDFromURL("https://example.com/watch?v=abcdefghijk"); err == "" {
		t.Fatal("expected reject")
	}
}

func TestIsYouTubeShortsURL(t *testing.T) {
	for _, raw := range []string{
		"https://www.youtube.com/shorts/abcdefghijk",
		"https://youtube.com/shorts/abcdefghijk?feature=share",
		"https://m.youtube.com/shorts/abcdefghijk",
		" https://www.youtube.com/shorts/abcdefghijk ",
	} {
		if !isYouTubeShortsURL(raw) {
			t.Fatalf("expected shorts: %s", raw)
		}
	}
	for _, raw := range []string{
		"https://www.youtube.com/watch?v=abcdefghijk",
		"https://youtu.be/abcdefghijk",
		"https://www.youtube.com/embed/abcdefghijk",
		"https://www.youtube.com/live/abcdefghijk",
		"https://www.youtube.com/shorts/",
		"https://example.com/shorts/abcdefghijk",
		"not a url",
		"",
	} {
		if isYouTubeShortsURL(raw) {
			t.Fatalf("expected not shorts: %s", raw)
		}
	}
}

func TestYouTubeThumbnailURL(t *testing.T) {
	if YouTubeThumbnailURL("https://i.ytimg.com/vi/abcdefghijk/hqdefault.jpg") != "" {
		t.Fatal("i.ytimg.com")
	}
	if YouTubeThumbnailURL("https://img.youtube.com/vi/abcdefghijk/0.jpg") != "" {
		t.Fatal("img.youtube.com")
	}
	for _, raw := range []string{
		"https://example.com/t.jpg",
		"http://i.ytimg.com/vi/abcdefghijk/hqdefault.jpg",
		"https://i.ytimg.com.evil.com/t.jpg",
		"https://user@i.ytimg.com/vi/abcdefghijk/hqdefault.jpg",
	} {
		if YouTubeThumbnailURL(raw) == "" {
			t.Fatalf("accepted %s", raw)
		}
	}
	bad := "https://example.com/t.jpg"
	id := "abcdefghijk"
	var post postDTO
	applyKind(&post, "youtube", &id, &bad, nil)
	if post.ThumbnailURL != "" {
		t.Fatalf("stored thumbnail leaked: %s", post.ThumbnailURL)
	}
}

func TestParseOEmbed(t *testing.T) {
	title, thumb, err := parseOEmbed("abcdefghijk", []byte(`{"title":"リレー","thumbnail_url":"https://i.ytimg.com/vi/abcdefghijk/hqdefault.jpg"}`))
	if err != nil || title != "リレー" || thumb != "https://i.ytimg.com/vi/abcdefghijk/hqdefault.jpg" {
		t.Fatalf("%q %q %v", title, thumb, err)
	}
	_, thumb, err = parseOEmbed("abcdefghijk", []byte(`{"title":"リレー","thumbnail_url":"https://example.com/t.jpg"}`))
	if err != nil || thumb != "https://i.ytimg.com/vi/abcdefghijk/hqdefault.jpg" {
		t.Fatalf("replaced %q %v", thumb, err)
	}
	long := strings.Repeat("あ", 130)
	title, _, err = parseOEmbed("abcdefghijk", []byte(`{"title":"`+long+`","thumbnail_url":""}`))
	if err != nil || len([]rune(title)) != 120 {
		t.Fatalf("clipped %d %v", len([]rune(title)), err)
	}
}
