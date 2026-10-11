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

func TestParseOEmbed(t *testing.T) {
	title, thumb, err := parseOEmbed("abcdefghijk", []byte(`{"title":"リレー","thumbnail_url":"https://i.ytimg.com/vi/abcdefghijk/hqdefault.jpg"}`))
	if err != nil || title != "リレー" || !strings.HasPrefix(thumb, "https://") {
		t.Fatalf("%q %q %v", title, thumb, err)
	}
	long := strings.Repeat("あ", 130)
	title, _, err = parseOEmbed("abcdefghijk", []byte(`{"title":"`+long+`","thumbnail_url":""}`))
	if err != nil || len([]rune(title)) != 120 {
		t.Fatalf("clipped %d %v", len([]rune(title)), err)
	}
}
