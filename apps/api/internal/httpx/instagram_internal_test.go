package httpx

import "testing"

func TestInstagramFromURL(t *testing.T) {
	cases := []struct {
		raw, kind, code string
	}{
		{"https://www.instagram.com/p/C1a2B3c4D5e/", "p", "C1a2B3c4D5e"},
		{"https://instagram.com/p/C1a2B3c4D5e?igsh=abc", "p", "C1a2B3c4D5e"},
		{"https://www.instagram.com/reel/C1a2B3c4D5e/?utm_source=ig_web_copy_link", "reel", "C1a2B3c4D5e"},
		{"https://www.instagram.com/reels/C1a2B3c4D5e/", "reel", "C1a2B3c4D5e"},
		{"https://m.instagram.com/p/C1a2B3c4D5e/embed/", "p", "C1a2B3c4D5e"},
		{"https://www.instagram.com/someone/p/C1a2B3c4D5e/", "p", "C1a2B3c4D5e"},
		{" https://www.instagram.com/someone/reel/C1a2B3c4D5e ", "reel", "C1a2B3c4D5e"},
	}
	for _, tc := range cases {
		kind, code, msg := InstagramFromURL(tc.raw)
		if msg != "" || kind != tc.kind || code != tc.code {
			t.Fatalf("%s -> %q %q %q", tc.raw, kind, code, msg)
		}
	}
	for _, raw := range []string{
		"",
		"instagram.com/p/C1a2B3c4D5e/",
		"javascript:alert(1)",
		"https://example.com/p/C1a2B3c4D5e/",
		"https://www.instagram.com.evil.com/p/C1a2B3c4D5e/",
		"https://evilinstagram.com/p/C1a2B3c4D5e/",
		"https://user@www.instagram.com/p/C1a2B3c4D5e/",
		"https://www.instagram.com:8443/p/C1a2B3c4D5e/",
		"https://www.instagram.com/someone/",
		"https://www.instagram.com/stories/someone/123/",
		"https://www.instagram.com/p/",
		"https://www.instagram.com/p/abc/",
		"https://www.instagram.com/p/C1a2%22onload%3D/",
		"https://www.instagram.com/p/C1a2B3c4D5e.js/",
	} {
		if _, _, msg := InstagramFromURL(raw); msg == "" {
			t.Fatalf("accepted %q", raw)
		}
	}
}

func TestApplyInstagramDropsBrokenValues(t *testing.T) {
	bad := "../evil"
	kind := "p"
	p := postDTO{Kind: "instagram"}
	applyInstagram(&p, &kind, &bad)
	if p.InstagramCode != "" || p.InstagramType != "" {
		t.Fatalf("leaked %+v", p)
	}
	good := "C1a2B3c4D5e"
	text := postDTO{Kind: "text"}
	applyInstagram(&text, &kind, &good)
	if text.InstagramCode != "" {
		t.Fatalf("text post got instagram %+v", text)
	}
	ig := postDTO{Kind: "instagram"}
	applyInstagram(&ig, &kind, &good)
	if ig.InstagramCode != good || ig.InstagramType != "p" {
		t.Fatalf("missing %+v", ig)
	}
}
