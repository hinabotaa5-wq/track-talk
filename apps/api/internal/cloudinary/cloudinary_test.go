package cloudinary

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
	"testing"
)

func TestSignOrdersParams(t *testing.T) {
	got := Sign(map[string]string{"timestamp": "1315060510", "public_id": "sample"}, "abcd")
	sum := sha1.Sum([]byte("public_id=sample&timestamp=1315060510abcd"))
	want := hex.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestVideoURLs(t *testing.T) {
	c := Client{Cloud: "demo"}
	id := "track-talk/11111111-1111-4111-8111-111111111111"
	thumb := c.ThumbnailURL(id)
	play := c.PlaybackURL(id)
	if !strings.Contains(thumb, "/demo/video/upload/so_auto,c_fill,w_640,h_480,f_jpg/track-talk/") || !strings.HasSuffix(thumb, ".jpg") {
		t.Fatal(thumb)
	}
	if !strings.Contains(play, "/demo/video/upload/q_auto/track-talk/") || !strings.HasSuffix(play, ".mp4") {
		t.Fatal(play)
	}
}

func TestImageURL(t *testing.T) {
	c := Client{Cloud: "demo"}
	id := "track-talk/11111111-1111-4111-8111-111111111111"
	got := c.ImageURL(id)
	if !strings.Contains(got, "/demo/image/upload/c_limit,w_1600,q_auto,f_auto/track-talk/") {
		t.Fatal(got)
	}
}

func TestValidPublicID(t *testing.T) {
	if !ValidPublicID("track-talk/11111111-1111-4111-8111-111111111111") {
		t.Fatal("expected valid")
	}
	if ValidPublicID("tracktalk/sample") || ValidPublicID("other/11111111-1111-4111-8111-111111111111") {
		t.Fatal("expected invalid")
	}
}
