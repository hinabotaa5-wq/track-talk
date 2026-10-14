package cloudinary

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"
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
	if !strings.Contains(thumb, "/demo/video/upload/so_auto,c_limit,w_800,f_jpg/track-talk/") || !strings.HasSuffix(thumb, ".jpg") {
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

func TestSignedUploadBindsPresetAndFormats(t *testing.T) {
	c := Client{Cloud: "demo", Key: "key", Secret: "secret"}
	now := time.Unix(1315060510, 0)
	image, err := c.SignedUpload(now, "image")
	if err != nil {
		t.Fatal(err)
	}
	if image.UploadPreset != ImagePreset || image.AllowedFormats != ImageFormats || image.ResourceType != "image" {
		t.Fatalf("image %+v", image)
	}
	want := Sign(map[string]string{
		"allowed_formats": image.AllowedFormats,
		"public_id":       image.PublicID,
		"timestamp":       "1315060510",
		"upload_preset":   image.UploadPreset,
	}, "secret")
	if image.Signature != want {
		t.Fatalf("signature %s want %s", image.Signature, want)
	}
	video, err := c.SignedUpload(now, "video")
	if err != nil {
		t.Fatal(err)
	}
	if video.UploadPreset != VideoPreset || video.AllowedFormats != VideoFormats {
		t.Fatalf("video %+v", video)
	}
	if _, err := c.SignedUpload(now, "raw"); err == nil {
		t.Fatal("expected invalid resource")
	}
}

func TestReadyRejectsOversizedAndWrongFormat(t *testing.T) {
	var payload string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()
	c := Client{Cloud: "demo", Key: "key", Secret: "secret", APIBase: srv.URL}
	id := "track-talk/11111111-1111-4111-8111-111111111111"
	ctx := context.Background()

	payload = `{"resource_type":"image","format":"jpg","bytes":1024}`
	if err := c.ImageReady(ctx, id); err != nil {
		t.Fatal(err)
	}
	payload = `{"resource_type":"image","format":"jpg","bytes":10485761}`
	if err := c.ImageReady(ctx, id); err != ErrTooLarge {
		t.Fatalf("size: %v", err)
	}
	payload = `{"resource_type":"image","format":"exe","bytes":1024}`
	if err := c.ImageReady(ctx, id); err != ErrRejected {
		t.Fatalf("format: %v", err)
	}
}

func TestEnsureUploadPresetsUpdatesAnExistingPreset(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "key" || pass != "secret" {
			http.Error(w, "auth", http.StatusUnauthorized)
			return
		}
		gotMethod = r.Method
		if r.Method == http.MethodPost {
			http.Error(w, "already exists", http.StatusBadRequest)
			return
		}
		if r.Method != http.MethodPut || !strings.Contains(r.URL.Path, ImagePreset) && !strings.Contains(r.URL.Path, VideoPreset) {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	c := Client{Cloud: "demo", Key: "key", Secret: "secret", APIBase: srv.URL}
	if err := c.EnsureUploadPresets(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPut {
		t.Fatalf("method %s", gotMethod)
	}
}

func TestEnsureUploadPresetsLive(t *testing.T) {
	if os.Getenv("CLOUDINARY_ENSURE") != "1" {
		t.Skip()
	}
	_ = godotenv.Load("../../../../.env")
	c := Client{
		Cloud:  os.Getenv("CLOUDINARY_CLOUD_NAME"),
		Key:    os.Getenv("CLOUDINARY_API_KEY"),
		Secret: os.Getenv("CLOUDINARY_API_SECRET"),
	}
	if !c.Configured() {
		t.Fatal("cloudinary credentials are missing")
	}
	if err := c.EnsureUploadPresets(context.Background()); err != nil {
		t.Fatal(err)
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
