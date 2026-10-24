package cloudinary

import (
	"context"
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
	want := "0d4fe14b2b4a3f68a97ccc5097c43908b623d24293c296826a9390c14d891509"
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
	if !strings.Contains(play, "/demo/video/upload/c_limit,w_1280,h_1280,q_auto/track-talk/") || !strings.HasSuffix(play, ".mp4") {
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
	transformations := map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "key" || pass != "secret" {
			http.Error(w, "auth", http.StatusUnauthorized)
			return
		}
		gotMethod = r.Method
		if err := r.ParseForm(); err == nil && r.Method == http.MethodPut {
			transformations[r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]] = r.PostForm.Get("transformation")
		}
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
	if transformations[ImagePreset] != ImageIncoming || transformations[VideoPreset] != VideoIncoming {
		t.Fatalf("transformations %v", transformations)
	}
}

func TestDeleteSendsPublicIDs(t *testing.T) {
	var gotPath string
	var gotIDs []string
	var status = http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "key" || pass != "secret" || r.Method != http.MethodDelete {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		gotPath = r.URL.Path
		gotIDs = r.URL.Query()["public_ids[]"]
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"deleted":{}}`))
	}))
	defer srv.Close()
	c := Client{Cloud: "demo", Key: "key", Secret: "secret", APIBase: srv.URL}
	ids := []string{"track-talk/11111111-1111-4111-8111-111111111111", "track-talk/22222222-2222-4222-8222-222222222222"}
	ctx := context.Background()
	if err := c.Delete(ctx, "image", ids); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v1_1/demo/resources/image/upload" || len(gotIDs) != 2 || gotIDs[0] != ids[0] || gotIDs[1] != ids[1] {
		t.Fatalf("path %s ids %v", gotPath, gotIDs)
	}
	if err := c.Delete(ctx, "video", []string{"other/sample"}); err != ErrInvalid {
		t.Fatalf("invalid id: %v", err)
	}
	status = http.StatusInternalServerError
	if err := c.Delete(ctx, "video", ids[:1]); err == nil {
		t.Fatal("expected error")
	}
	if err := (Client{}).Delete(ctx, "image", ids); err != nil {
		t.Fatalf("not configured: %v", err)
	}

	// 非公開の動画は authenticated から消す。公開側の ID は受け付けない。
	status = http.StatusOK
	coaching := []string{CoachingFolder + "/33333333-3333-4333-8333-333333333333"}
	if err := c.DeleteCoachingVideos(ctx, coaching); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v1_1/demo/resources/video/authenticated" || len(gotIDs) != 1 || gotIDs[0] != coaching[0] {
		t.Fatalf("coaching path %s ids %v", gotPath, gotIDs)
	}
	if err := c.DeleteCoachingVideos(ctx, ids[:1]); err != ErrInvalid {
		t.Fatalf("public id as coaching: %v", err)
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

func TestCoachingUploadIsAuthenticated(t *testing.T) {
	c := Client{Cloud: "demo", Key: "key", Secret: "secret"}
	signed, err := c.SignedCoachingUpload(time.Unix(1700000000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if !ValidCoachingPublicID(signed.PublicID) || ValidPublicID(signed.PublicID) {
		t.Fatalf("public id %q", signed.PublicID)
	}
	if signed.Type != "authenticated" || signed.ResourceType != "video" {
		t.Fatalf("signed %+v", signed)
	}
	want := Sign(map[string]string{
		"allowed_formats": VideoFormats,
		"public_id":       signed.PublicID,
		"timestamp":       "1700000000",
		"type":            "authenticated",
		"upload_preset":   VideoPreset,
	}, "secret")
	if signed.Signature != want {
		t.Fatalf("signature %s want %s", signed.Signature, want)
	}
	if _, err := (Client{}).SignedCoachingUpload(time.Now()); err != ErrNotConfigured {
		t.Fatalf("unconfigured: %v", err)
	}
}

func TestCoachingURLsAreSigned(t *testing.T) {
	c := Client{Cloud: "demo", Key: "key", Secret: "secret"}
	id := "track-talk-coaching/0b8f0f5e-0c39-4f7e-9d3a-5f0b5d6c7a11"
	// 期待値は Cloudinary の公式 SDK と同じ手順（SHA-256 → URL 用 Base64 の先頭8文字）で別に計算したもの。
	if got, want := c.CoachingPlaybackURL(id), "https://res.cloudinary.com/demo/video/authenticated/s--OE0d7T-_--/q_auto,du_30/v1/"+id+".mp4"; got != want {
		t.Fatalf("playback\n got %s\nwant %s", got, want)
	}
	if got, want := c.CoachingThumbnailURL(id), "https://res.cloudinary.com/demo/video/authenticated/s--AF2cg9FF--/so_auto,c_limit,w_800,f_jpg/v1/"+id+".jpg"; got != want {
		t.Fatalf("thumbnail\n got %s\nwant %s", got, want)
	}
	if c.CoachingPlaybackURL("track-talk/0b8f0f5e-0c39-4f7e-9d3a-5f0b5d6c7a11") != "" {
		t.Fatal("public folder must not be signed as coaching")
	}
}

func TestCoachingVideoReadyChecksTypeAndLength(t *testing.T) {
	var payload, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()
	c := Client{Cloud: "demo", Key: "key", Secret: "secret", APIBase: srv.URL}
	id := "track-talk-coaching/11111111-1111-4111-8111-111111111111"
	ctx := context.Background()

	payload = `{"resource_type":"video","type":"authenticated","format":"mp4","bytes":1024,"duration":29.9}`
	if err := c.CoachingVideoReady(ctx, id); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(path, "/resources/video/authenticated/track-talk-coaching/") {
		t.Fatalf("path %s", path)
	}
	payload = `{"resource_type":"video","type":"authenticated","format":"mp4","bytes":1024,"duration":31}`
	if err := c.CoachingVideoReady(ctx, id); err != ErrTooLong {
		t.Fatalf("length: %v", err)
	}
	payload = `{"resource_type":"video","type":"upload","format":"mp4","bytes":1024,"duration":10}`
	if err := c.CoachingVideoReady(ctx, id); err != ErrRejected {
		t.Fatalf("public type: %v", err)
	}
	payload = `{"resource_type":"video","type":"authenticated","format":"mp4","bytes":104857601,"duration":10}`
	if err := c.CoachingVideoReady(ctx, id); err != ErrTooLarge {
		t.Fatalf("size: %v", err)
	}
	if err := c.CoachingVideoReady(ctx, "track-talk/11111111-1111-4111-8111-111111111111"); err != ErrInvalid {
		t.Fatalf("folder: %v", err)
	}
}

// トークン認証の鍵があれば、非公開の動画の URL に期限付きのトークンを付け、その動画だけに使えるようにする。
func TestCoachingURLsCarryExpiringTokenWithAuthKey(t *testing.T) {
	c := Client{
		Cloud: "demo", Key: "key", Secret: "secret",
		AuthTokenKey: "00112233445566778899aabbccddeeff",
		Now:          func() time.Time { return time.Unix(1800000000, 0).Add(-AuthTokenTTL) },
	}
	id := "track-talk-coaching/0b8f0f5e-0c39-4f7e-9d3a-5f0b5d6c7a11"
	// 期待値は Cloudinary の公式 Python SDK（cloudinary_url の auth_token）で、同じ鍵・期限・acl から作ったもの。
	token := "?__cld_token__=exp=1800000000~acl=%2fvideo%2fauthenticated%2f*track-talk-coaching%2f0b8f0f5e-0c39-4f7e-9d3a-5f0b5d6c7a11*~hmac=d4aea924058f847b18a5c8ed9a4a9b1e114fbdec9421de90650d1e185c53f7d8"
	if got, want := c.CoachingPlaybackURL(id), "https://res.cloudinary.com/demo/video/authenticated/q_auto,du_30/v1/"+id+".mp4"+token; got != want {
		t.Fatalf("playback\n got %s\nwant %s", got, want)
	}
	if got, want := c.CoachingThumbnailURL(id), "https://res.cloudinary.com/demo/video/authenticated/so_auto,c_limit,w_800,f_jpg/v1/"+id+".jpg"+token; got != want {
		t.Fatalf("thumbnail\n got %s\nwant %s", got, want)
	}
	c.AuthTokenKey = "not-hex"
	if c.CoachingPlaybackURL(id) != "" {
		t.Fatal("a broken key must not fall back to a URL without expiry")
	}
}
