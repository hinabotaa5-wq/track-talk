package httpx_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/auth"
	"github.com/hinata/track-talk/apps/api/internal/cloudinary"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/httpx"
	"github.com/hinata/track-talk/apps/api/internal/migrate"
	"github.com/jackc/pgx/v5/pgxpool"
)

func setup(t *testing.T) http.Handler {
	return setupConfig(t, nil)
}

// testPoolMaxConns が 0 でなければ、setupConfig が作るサーバーの接続の数をそれに絞る。
var testPoolMaxConns int32

func setupConfig(t *testing.T, mutate func(*config.Config)) http.Handler {
	t.Helper()
	dsn := testDSN(t)
	secret := os.Getenv("SUPABASE_JWT_SECRET")
	if secret == "" {
		secret = "test-jwt-secret-for-ci-only"
	}
	ctx := context.Background()
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("pool config: %v", err)
	}
	if testPoolMaxConns > 0 {
		poolCfg.MaxConns = testPoolMaxConns
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	if err := migrate.Up(dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	_, _ = pool.Exec(ctx, `TRUNCATE sense_summary_attempts, notifications, inquiries, reports, post_saves, post_likes, comments, practice_ratings, practice_applications, posts, profiles CASCADE`)
	v, err := auth.NewVerifier(secret, "", "authenticated")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		CORSOrigins: []string{"http://localhost:3000"},
		JWTSecret:   secret,
		JWTAUD:      "authenticated",
		DatabaseURL: dsn,
		// 本番は空（コーチングを閉じた状態）が既定だが、テストはコーチングの機能を
		// 確かめるものがほとんどなので開けておく。閉じたときの 404 は coaching_test.go で確かめる。
		CoachingEnabled:       true,
		CoachingNewAcceptance: true,
		CoachingMessaging:     true,
		// 本番はチケットの新規販売を止めている（空が既定）。購入済みチケットの消費・返金を確かめる
		// テストが多いので開けておき、止めたときの動きは coaching_ticket_sales_test.go で確かめる。
		CoachingTicketSales: true,
		// テスト用の鍵(32バイトの 0x01)。本番の鍵は Secret Manager に置く。
		CoachIdentityKey: "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=",
		CloudName:        os.Getenv("CLOUDINARY_CLOUD_NAME"),
		CloudAPIKey:      os.Getenv("CLOUDINARY_API_KEY"),
		CloudAPISecret:   os.Getenv("CLOUDINARY_API_SECRET"),
		CloudAPIBase:     os.Getenv("CLOUDINARY_API_BASE"),
		// 保護者の同意のリンクは、Resend の代わりのサーバーに送る（mail_helper_test.go）。
		ResendAPIKey: "re_test",
		MailFrom:     "TrackTalk <no-reply@example.com>",
		MailAPIBase:  mailServerURL(),
	}
	if mutate != nil {
		mutate(&cfg)
	}
	return httpx.New(cfg, pool, v)
}

func token(t *testing.T, id uuid.UUID) string {
	t.Helper()
	return tokenAt(t, id, time.Now())
}

func tokenAt(t *testing.T, id uuid.UUID, authTime time.Time) string {
	t.Helper()
	secret := os.Getenv("SUPABASE_JWT_SECRET")
	if secret == "" {
		secret = "test-jwt-secret-for-ci-only"
	}
	claims := jwt.MapClaims{
		"sub": id.String(),
		"aud": "authenticated",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	if !authTime.IsZero() {
		claims["auth_time"] = authTime.Unix()
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func tokenWithProfile(t *testing.T, id uuid.UUID, nickname string, public bool) string {
	t.Helper()
	secret := os.Getenv("SUPABASE_JWT_SECRET")
	if secret == "" {
		secret = "test-jwt-secret-for-ci-only"
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": id.String(),
		"aud": "authenticated",
		"exp": time.Now().Add(time.Hour).Unix(),
		"user_metadata": map[string]any{
			"nickname":        nickname,
			"nickname_public": public,
		},
	})
	s, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func doJSON(t *testing.T, h http.Handler, method, path, tok string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealth(t *testing.T) {
	h := setup(t)
	rec := doJSON(t, h, http.MethodGet, "/health", "", nil)
	if rec.Code != 200 {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
}

func TestUnauthorized(t *testing.T) {
	h := setup(t)
	rec := doJSON(t, h, http.MethodGet, "/v1/me", "", nil)
	if rec.Code != 401 {
		t.Fatalf("status %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me", "not-a-jwt", nil)
	if rec.Code != 401 {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestProfileNicknameAndPostVisibility(t *testing.T) {
	h := setup(t)
	userID := uuid.New()
	tok := tokenWithProfile(t, userID, "初期ネーム", true)

	rec := doJSON(t, h, http.MethodGet, "/v1/me", tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("me %d %s", rec.Code, rec.Body.String())
	}
	var profile struct {
		Nickname       string `json:"nickname"`
		NicknamePublic bool   `json:"nickname_public"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &profile)
	if profile.Nickname != "初期ネーム" || !profile.NicknamePublic {
		t.Fatalf("profile %+v", profile)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts", tok, map[string]string{
		"title": "公開名の投稿", "body": "本文",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	rec = doJSON(t, h, http.MethodGet, "/v1/posts/"+created.ID, "", nil)
	var post struct {
		AuthorName *string `json:"author_name"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &post)
	if rec.Code != http.StatusOK || post.AuthorName == nil || *post.AuthorName != "初期ネーム" {
		t.Fatalf("public post %d %+v", rec.Code, post)
	}

	rec = doJSON(t, h, http.MethodPatch, "/v1/me", tok, map[string]any{
		"nickname": "変更後", "nickname_public": false,
	})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("update without event %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPatch, "/v1/me", tok, map[string]any{
		"nickname": "変更後", "nickname_public": false, "event": "marathon",
	})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("update with unknown event %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPatch, "/v1/me", tok, map[string]any{
		"nickname": "変更後", "nickname_public": false, "event": "100m",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("update %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/posts/"+created.ID, "", nil)
	post.AuthorName = nil
	_ = json.Unmarshal(rec.Body.Bytes(), &post)
	if rec.Code != http.StatusOK || post.AuthorName != nil {
		t.Fatalf("private post %d %+v", rec.Code, post)
	}

	// 種目を選んだあとは、ニックネームだけ送っても種目は変わらない。
	rec = doJSON(t, h, http.MethodPatch, "/v1/me", tok, map[string]any{
		"nickname": "また変更", "nickname_public": false,
	})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"event":"100m"`) {
		t.Fatalf("keep event %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPatch, "/v1/me", tok, map[string]any{
		"nickname": "", "nickname_public": true,
	})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid nickname %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPatch, "/v1/me", tok, map[string]any{
		"nickname": "12345678901", "nickname_public": true,
	})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("long nickname %d %s", rec.Code, rec.Body.String())
	}
}

func TestPostLifecycleAndAuthz(t *testing.T) {
	h := setup(t)
	a := uuid.New()
	b := uuid.New()
	ta, tb := token(t, a), token(t, b)

	rec := doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{
		"title": "日本選手権 100m", "body": "決勝の展開が熱かった"})
	if rec.Code != 201 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	rec = doJSON(t, h, http.MethodPatch, "/v1/posts/"+created.ID, tb, map[string]any{
		"title": "改ざん", "body": "だめ"})
	if rec.Code != 403 {
		t.Fatalf("other edit %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/likes", tb, nil)
	if rec.Code != 204 {
		t.Fatalf("like %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/comments", tb, map[string]string{"body": "同感です"})
	if rec.Code != 201 {
		t.Fatalf("comment %d %s", rec.Code, rec.Body.String())
	}
	var cmt struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &cmt)

	rec = doJSON(t, h, http.MethodPatch, "/v1/comments/"+cmt.ID, ta, map[string]string{"body": "他人"})
	if rec.Code != 403 {
		t.Fatalf("other comment edit %d", rec.Code)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/reports", ta, map[string]string{
		"target_type": "post", "target_id": created.ID, "reason": "spam", "detail": "",
	})
	if rec.Code != 201 {
		t.Fatalf("report %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/reports", ta, map[string]string{
		"target_type": "post", "target_id": created.ID, "reason": "spam", "detail": "",
	})
	if rec.Code != 409 {
		t.Fatalf("dup report %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/posts?sort=popular", "", nil)
	if rec.Code != 200 {
		t.Fatalf("list %d", rec.Code)
	}

	rec = doJSON(t, h, http.MethodDelete, "/v1/posts/"+created.ID, ta, nil)
	if rec.Code != 204 {
		t.Fatalf("delete %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/posts/"+created.ID, "", nil)
	if rec.Code != 404 {
		t.Fatalf("deleted visible %d", rec.Code)
	}
}

func TestPostKindsShareOneList(t *testing.T) {
	h := setup(t)
	a := uuid.New()
	ta := token(t, a)
	rec := doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{
		"title": "", "body": "テキスト"})
	if rec.Code != 201 {
		t.Fatalf("text %d %s", rec.Code, rec.Body.String())
	}
	var textPost struct {
		Kind string `json:"kind"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &textPost)
	if textPost.Kind != "text" {
		t.Fatalf("kind %q", textPost.Kind)
	}

	yt := map[string]any{
		"title": "公式", "body": "", "kind": "youtube",
		"youtube_video_id": "abcdefghijk", "youtube_thumbnail_url": "https://i.ytimg.com/vi/abcdefghijk/hqdefault.jpg",
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts", ta, yt)
	if rec.Code != 201 {
		t.Fatalf("youtube %d %s", rec.Code, rec.Body.String())
	}
	var ytPost struct {
		Kind         string `json:"kind"`
		ThumbnailURL string `json:"thumbnail_url"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &ytPost)
	if ytPost.Kind != "youtube" || ytPost.ThumbnailURL == "" {
		t.Fatalf("youtube dto %+v", ytPost)
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts", ta, yt)
	if rec.Code != 409 {
		t.Fatalf("dup youtube %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{
		"title": "自分の動画", "body": "", "kind": "video",
		"video_public_id": "tracktalk/sample",
	})
	if rec.Code != 422 {
		t.Fatalf("video %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/posts?sort=new", "", nil)
	if rec.Code != 200 {
		t.Fatalf("list %d", rec.Code)
	}
	var list struct {
		Posts []struct {
			Kind string `json:"kind"`
		} `json:"posts"`
		Meta struct {
			Total int64 `json:"total"`
		} `json:"meta"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if list.Meta.Total != 2 {
		t.Fatalf("total %d", list.Meta.Total)
	}
	seen := map[string]bool{}
	for _, p := range list.Posts {
		seen[p.Kind] = true
	}
	if !seen["text"] || !seen["youtube"] || seen["video"] {
		t.Fatalf("kinds %#v", seen)
	}
}

func TestTextPostRequiresBodyOrPhoto(t *testing.T) {
	h := setup(t)
	ta := token(t, uuid.New())

	for name, payload := range map[string]map[string]any{
		"body only": {"title": "", "body": "本文だけ"},
	} {
		t.Run(name, func(t *testing.T) {
			rec := doJSON(t, h, http.MethodPost, "/v1/posts", ta, payload)
			if rec.Code != 201 {
				t.Fatalf("create %d %s", rec.Code, rec.Body.String())
			}
		})
	}
	rec := doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{
		"title": "タイトルだけ", "body": "",
	})
	if rec.Code != 422 {
		t.Fatalf("title only %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{
		"title": " \t", "body": "\n",
	})
	if rec.Code != 422 {
		t.Fatalf("empty text %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{
		"title": "", "body": "編集前の本文",
	})
	if rec.Code != 201 {
		t.Fatalf("create for update %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	rec = doJSON(t, h, http.MethodPatch, "/v1/posts/"+created.ID, ta, map[string]any{
		"title": "", "body": "",
	})
	if rec.Code != 422 {
		t.Fatalf("empty update %d %s", rec.Code, rec.Body.String())
	}
}

func TestSignedVideoJoinsTheList(t *testing.T) {
	var assetBytes atomic.Int64
	assetBytes.Store(1024)
	asset := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "key" || pass != "secret" {
			http.Error(w, "auth", http.StatusUnauthorized)
			return
		}
		if !strings.Contains(r.URL.Path, "/resources/video/upload/track-talk/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"resource_type":"video","format":"mp4","bytes":%d}`, assetBytes.Load())
	}))
	defer asset.Close()
	t.Setenv("CLOUDINARY_CLOUD_NAME", "demo")
	t.Setenv("CLOUDINARY_API_KEY", "key")
	t.Setenv("CLOUDINARY_API_SECRET", "secret")
	t.Setenv("CLOUDINARY_API_BASE", asset.URL)

	h := setup(t)
	a := uuid.New()
	ta := token(t, a)
	rec := doJSON(t, h, http.MethodPost, "/v1/uploads/video", ta, map[string]any{})
	if rec.Code != 200 {
		t.Fatalf("sign %d %s", rec.Code, rec.Body.String())
	}
	var signed struct {
		CloudName      string `json:"cloud_name"`
		APIKey         string `json:"api_key"`
		PublicID       string `json:"public_id"`
		Signature      string `json:"signature"`
		Timestamp      int64  `json:"timestamp"`
		UploadPreset   string `json:"upload_preset"`
		AllowedFormats string `json:"allowed_formats"`
		ResourceType   string `json:"resource_type"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &signed)
	if signed.CloudName != "demo" || signed.APIKey != "key" || !cloudinary.ValidPublicID(signed.PublicID) {
		t.Fatalf("signed %+v", signed)
	}
	if signed.UploadPreset != cloudinary.VideoPreset || signed.AllowedFormats != cloudinary.VideoFormats || signed.ResourceType != "video" {
		t.Fatalf("constraints %+v", signed)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("secret")) {
		t.Fatal("secret leaked")
	}
	want := cloudinary.Sign(map[string]string{
		"allowed_formats": signed.AllowedFormats,
		"public_id":       signed.PublicID,
		"timestamp":       strconv.FormatInt(signed.Timestamp, 10),
		"upload_preset":   signed.UploadPreset,
	}, "secret")
	if signed.Signature != want {
		t.Fatalf("signature %s want %s", signed.Signature, want)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{
		"title": "", "body": "走った動画の詳細", "kind": "video",
		"video_public_id": signed.PublicID,
	})
	if rec.Code != 201 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Title        string `json:"title"`
		Body         string `json:"body"`
		Kind         string `json:"kind"`
		ThumbnailURL string `json:"thumbnail_url"`
		PlaybackURL  string `json:"playback_url"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.Title != "" || created.Body != "走った動画の詳細" || created.Kind != "video" || created.ThumbnailURL == "" || created.PlaybackURL == "" {
		t.Fatalf("created %+v", created)
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/posts?sort=new", "", nil)
	var videoList struct {
		Posts []struct {
			Kind    string `json:"kind"`
			Title   string `json:"title"`
			Excerpt string `json:"excerpt"`
		} `json:"posts"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &videoList)
	if len(videoList.Posts) == 0 || videoList.Posts[0].Kind != "video" || videoList.Posts[0].Title != "" || videoList.Posts[0].Excerpt != "走った動画の詳細" {
		t.Fatalf("video feed = %s", rec.Body.String())
	}

	assetBytes.Store(cloudinary.MaxVideoBytes + 1)
	rec = doJSON(t, h, http.MethodPost, "/v1/uploads/video", ta, map[string]any{})
	if rec.Code != 200 {
		t.Fatalf("sign large %d %s", rec.Code, rec.Body.String())
	}
	var large struct {
		PublicID string `json:"public_id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &large)
	rec = doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{
		"title": "", "body": "", "kind": "video",
		"video_public_id": large.PublicID,
	})
	if rec.Code != 422 {
		t.Fatalf("large video %d %s", rec.Code, rec.Body.String())
	}
}

func TestSignedImageJoinsTheList(t *testing.T) {
	asset := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "key" || pass != "secret" {
			http.Error(w, "auth", http.StatusUnauthorized)
			return
		}
		if !strings.Contains(r.URL.Path, "/resources/image/upload/track-talk/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"resource_type":"image","format":"jpg","bytes":1024}`))
	}))
	defer asset.Close()
	t.Setenv("CLOUDINARY_CLOUD_NAME", "demo")
	t.Setenv("CLOUDINARY_API_KEY", "key")
	t.Setenv("CLOUDINARY_API_SECRET", "secret")
	t.Setenv("CLOUDINARY_API_BASE", asset.URL)

	h := setup(t)
	a := uuid.New()
	ta := token(t, a)
	rec := doJSON(t, h, http.MethodPost, "/v1/uploads/image", ta, map[string]any{})
	if rec.Code != 200 {
		t.Fatalf("sign %d %s", rec.Code, rec.Body.String())
	}
	var signed struct {
		PublicID string `json:"public_id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &signed)
	if !cloudinary.ValidPublicID(signed.PublicID) {
		t.Fatalf("signed %+v", signed)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{
		"title": "写真タイトル", "body": "", "image_public_id": signed.PublicID,
	})
	if rec.Code != 201 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID            string `json:"id"`
		Kind          string `json:"kind"`
		ThumbnailURL  string `json:"thumbnail_url"`
		PlaybackURL   string `json:"playback_url"`
		ImagePublicID string `json:"image_public_id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.Kind != "text" || created.ImagePublicID != signed.PublicID || created.PlaybackURL != "" || !strings.Contains(created.ThumbnailURL, "/image/upload/") {
		t.Fatalf("created %+v", created)
	}

	rec = doJSON(t, h, http.MethodPatch, "/v1/posts/"+created.ID, ta, map[string]any{
		"title": "写真タイトル編集", "body": "",
	})
	if rec.Code != 200 {
		t.Fatalf("update image-only post %d %s", rec.Code, rec.Body.String())
	}
}

func TestCiteYouTubeOpensExistingThread(t *testing.T) {
	h := setup(t)
	a := uuid.New()
	ta := token(t, a)
	rec := doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{
		"title": "公式", "body": "", "kind": "youtube",
		"youtube_video_id": "abcdefghijk", "youtube_thumbnail_url": "https://i.ytimg.com/vi/abcdefghijk/hqdefault.jpg",
	})
	if rec.Code != 201 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	rec = doJSON(t, h, http.MethodPost, "/v1/posts/youtube", ta, map[string]any{
		"url": "https://youtu.be/abcdefghijk", "title": "引用タイトル"})
	if rec.Code != 200 {
		t.Fatalf("cite %d %s", rec.Code, rec.Body.String())
	}
	var cited struct {
		Existing bool `json:"existing"`
		Post     struct {
			ID string `json:"id"`
		} `json:"post"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &cited)
	if !cited.Existing || cited.Post.ID != created.ID {
		t.Fatalf("cited %+v want %s", cited, created.ID)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts/youtube", ta, map[string]any{
		"url": "https://youtu.be/abcdefghijk", "title": "別の名前"})
	if rec.Code != 200 {
		t.Fatalf("cite with title %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts/youtube", ta, map[string]any{
		"url": "https://example.com/watch?v=abcdefghijk"})
	if rec.Code != 422 {
		t.Fatalf("bad url %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts/youtube", ta, map[string]any{
		"url": "https://youtu.be/zzzzzzzzzzz", "title": strings.Repeat("あ", 121)})
	if rec.Code != 422 {
		t.Fatalf("long title %d %s", rec.Code, rec.Body.String())
	}
}

func TestValidation(t *testing.T) {
	h := setup(t)
	a := uuid.New()
	ta := token(t, a)
	rec := doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{"title": strings.Repeat("あ", 121), "body": "本文"})
	if rec.Code != 422 {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{
		"kind": "youtube", "title": "動画", "body": "",
		"youtube_video_id": "abcdefghijk", "youtube_thumbnail_url": "https://example.com/t.jpg",
	})
	if rec.Code != 422 {
		t.Fatalf("thumbnail %d %s", rec.Code, rec.Body.String())
	}
}

func TestSaveAppearsOnlyForTheSaver(t *testing.T) {
	h := setup(t)
	a := uuid.New()
	b := uuid.New()
	ta, tb := token(t, a), token(t, b)
	rec := doJSON(t, h, http.MethodGet, "/v1/me/saves", "", nil)
	if rec.Code != 401 {
		t.Fatalf("anon %d", rec.Code)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{"title": "保存するレース", "body": "決勝"})
	if rec.Code != 201 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/saves", tb, nil)
	if rec.Code != 204 {
		t.Fatalf("save %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/saves", tb, nil)
	if rec.Code != 204 {
		t.Fatalf("save again %d", rec.Code)
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/posts/"+created.ID, tb, nil)
	if rec.Code != 200 {
		t.Fatalf("get %d", rec.Code)
	}
	var got struct {
		SavedByMe bool `json:"saved_by_me"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if !got.SavedByMe {
		t.Fatal("expected saved")
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/me/saves?q="+url.QueryEscape("決勝"), tb, nil)
	if rec.Code != 200 {
		t.Fatalf("list %d %s", rec.Code, rec.Body.String())
	}
	var list struct {
		Posts []struct {
			ID        string `json:"id"`
			SavedByMe bool   `json:"saved_by_me"`
		} `json:"posts"`
		Meta struct {
			Total int64 `json:"total"`
		} `json:"meta"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if list.Meta.Total != 1 || len(list.Posts) != 1 || list.Posts[0].ID != created.ID || !list.Posts[0].SavedByMe {
		t.Fatalf("list %+v", list)
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/me/saves", ta, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if list.Meta.Total != 0 {
		t.Fatalf("author list %+v", list)
	}

	rec = doJSON(t, h, http.MethodDelete, "/v1/posts/"+created.ID+"/saves", tb, nil)
	if rec.Code != 204 {
		t.Fatalf("unsave %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me/saves", tb, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if list.Meta.Total != 0 {
		t.Fatalf("after unsave %+v", list)
	}
}

func TestInquiryStaysWithTheSender(t *testing.T) {
	h := setup(t)
	a := uuid.New()
	b := uuid.New()
	ta, tb := token(t, a), token(t, b)

	rec := doJSON(t, h, http.MethodPost, "/v1/me/inquiries", "", map[string]string{
		"category": "bug", "subject": "件名", "body": "本文",
	})
	if rec.Code != 401 {
		t.Fatalf("anon %d", rec.Code)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/me/inquiries", ta, map[string]string{
		"category": "nope", "subject": "", "body": "",
	})
	if rec.Code != 422 {
		t.Fatalf("invalid %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/me/inquiries", ta, map[string]string{
		"category": "bug", "subject": "ログインできない", "body": "パスワード再設定後も入れません",
	})
	if rec.Code != 422 {
		t.Fatalf("missing email %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/me/inquiries", ta, map[string]string{
		"category": "bug", "subject": "ログインできない", "body": "パスワード再設定後も入れません", "email": "not-an-email",
	})
	if rec.Code != 422 {
		t.Fatalf("bad email %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/me/inquiries", ta, map[string]string{
		"category": "bug", "subject": "ログインできない", "body": "パスワード再設定後も入れません", "email": "runner@example.com",
	})
	if rec.Code != 201 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/me/inquiries", ta, nil)
	if rec.Code != 200 {
		t.Fatalf("mine %d %s", rec.Code, rec.Body.String())
	}
	var list struct {
		Inquiries []struct {
			Subject      string  `json:"subject"`
			UserID       string  `json:"user_id"`
			ContactEmail *string `json:"contact_email"`
		} `json:"inquiries"`
		Meta struct {
			Total int64 `json:"total"`
		} `json:"meta"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if list.Meta.Total != 1 || len(list.Inquiries) != 1 || list.Inquiries[0].Subject != "ログインできない" || list.Inquiries[0].UserID != a.String() ||
		list.Inquiries[0].ContactEmail == nil || *list.Inquiries[0].ContactEmail != "runner@example.com" {
		t.Fatalf("mine %+v", list)
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/me/inquiries", tb, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if list.Meta.Total != 0 || len(list.Inquiries) != 0 {
		t.Fatalf("other %+v", list)
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/admin/inquiries", ta, nil)
	if rec.Code != 403 {
		t.Fatalf("admin forbidden %d %s", rec.Code, rec.Body.String())
	}

	setAdmin(t, a)
	rec = doJSON(t, h, http.MethodGet, "/v1/admin/inquiries", ta, nil)
	if rec.Code != 200 {
		t.Fatalf("admin %d %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if list.Meta.Total != 1 || len(list.Inquiries) != 1 || list.Inquiries[0].Subject != "ログインできない" {
		t.Fatalf("admin list %+v", list)
	}
}

func TestRecruitmentAppearsInTheFeed(t *testing.T) {
	asset := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "key" || pass != "secret" {
			http.Error(w, "auth", http.StatusUnauthorized)
			return
		}
		if !strings.Contains(r.URL.Path, "/resources/image/upload/track-talk/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"resource_type":"image","format":"jpg","bytes":1024}`))
	}))
	defer asset.Close()
	t.Setenv("CLOUDINARY_CLOUD_NAME", "demo")
	t.Setenv("CLOUDINARY_API_KEY", "key")
	t.Setenv("CLOUDINARY_API_SECRET", "secret")
	t.Setenv("CLOUDINARY_API_BASE", asset.URL)

	h := setup(t)
	a := uuid.New()
	b := uuid.New()
	ta, tb := token(t, a), token(t, b)

	rec := doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{
		"title": "城南クラブ", "body": "初心者も歓迎です", "kind": "recruitment",
		"schedule": "毎週火曜 19:00-21:00",
	})
	if rec.Code != 422 {
		t.Fatalf("missing prefecture %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/uploads/image", ta, map[string]any{})
	if rec.Code != 200 {
		t.Fatalf("sign %d %s", rec.Code, rec.Body.String())
	}
	var signed struct {
		PublicID string `json:"public_id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &signed)

	rec = doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{
		"title": "城南クラブ", "body": "初心者も歓迎です", "kind": "recruitment",
		"prefectures": []string{"13", "14"}, "venue": "駒沢競技場",
		"schedule": "毎週火曜 19:00-21:00", "gender_ratio": "男性7：女性3",
		"average_age": "25歳", "recruit_age": "18〜30歳",
		"links":            []string{"https://example.com/team"},
		"image_public_ids": []string{signed.PublicID},
	})
	if rec.Code != 201 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID           string `json:"id"`
		Kind         string `json:"kind"`
		ThumbnailURL string `json:"thumbnail_url"`
		Recruitment  struct {
			Prefectures []string `json:"prefectures"`
			Venue       string   `json:"venue"`
			Links       []string `json:"links"`
			Photos      []struct {
				PublicID string `json:"public_id"`
			} `json:"photos"`
		} `json:"recruitment"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.Kind != "recruitment" || created.ThumbnailURL == "" || created.Recruitment.Venue != "駒沢競技場" || len(created.Recruitment.Photos) != 1 {
		t.Fatalf("created %+v", created)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{
		"title": "文章", "body": "レースの話",
	})
	if rec.Code != 201 {
		t.Fatalf("text %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/posts?sort=new&kind=recruitment", "", nil)
	if rec.Code != 200 {
		t.Fatalf("filter %d", rec.Code)
	}
	var filtered struct {
		Posts []struct {
			Kind         string `json:"kind"`
			ThumbnailURL string `json:"thumbnail_url"`
		} `json:"posts"`
		Meta struct {
			Total int64 `json:"total"`
		} `json:"meta"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &filtered)
	if filtered.Meta.Total != 1 || filtered.Posts[0].Kind != "recruitment" || filtered.Posts[0].ThumbnailURL == "" {
		t.Fatalf("filtered %+v", filtered)
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/posts?sort=new", "", nil)
	var mixed struct {
		Meta struct {
			Total int64 `json:"total"`
		} `json:"meta"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &mixed)
	if mixed.Meta.Total != 2 {
		t.Fatalf("mixed %d", mixed.Meta.Total)
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/posts?sort=new&q="+url.QueryEscape("東京"), "", nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &filtered)
	if filtered.Meta.Total != 1 {
		t.Fatalf("prefecture search %+v", filtered)
	}

	rec = doJSON(t, h, http.MethodPatch, "/v1/posts/"+created.ID, tb, map[string]any{
		"title": "改ざん", "body": "だめ", "kind": "recruitment",
		"prefectures": []string{"13"}, "schedule": "毎週火曜",
	})
	if rec.Code != 403 {
		t.Fatalf("other edit %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPatch, "/v1/posts/"+created.ID, ta, map[string]any{
		"title": "城南トラッククラブ", "body": "社会人も歓迎です", "kind": "recruitment",
		"prefectures": []string{"13"}, "venue": "",
		"schedule": "毎週土曜 9:00-12:00",
		"links":    []string{"https://example.com/a", "https://example.com/b"},
	})
	if rec.Code != 200 {
		t.Fatalf("update %d %s", rec.Code, rec.Body.String())
	}
	var updated struct {
		Title       string `json:"title"`
		Recruitment struct {
			Venue  string   `json:"venue"`
			Links  []string `json:"links"`
			Photos []struct {
				PublicID string `json:"public_id"`
			} `json:"photos"`
		} `json:"recruitment"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.Title != "城南トラッククラブ" || updated.Recruitment.Venue != "" || len(updated.Recruitment.Links) != 2 || len(updated.Recruitment.Photos) != 0 {
		t.Fatalf("updated %+v", updated)
	}
}

func TestPracticeSharesTheRecruitmentFeed(t *testing.T) {
	h := setup(t)
	a := uuid.New()
	b := uuid.New()
	ta, tb := token(t, a), token(t, b)
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	starts := time.Now().In(loc).Add(48 * time.Hour)
	starts = time.Date(starts.Year(), starts.Month(), starts.Day(), starts.Hour(), starts.Minute(), 0, 0, loc)

	rec := doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{
		"kind": "practice", "body": "ジョグから入ります",
	})
	if rec.Code != 422 {
		t.Fatalf("missing fields %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{
		"kind": "practice", "body": "ジョグから入ります", "venue": "駒沢競技場",
		"starts_at": time.Now().Add(-time.Hour).Format(time.RFC3339),
		"event":     "100m", "capacity": 3,
	})
	if rec.Code != 422 {
		t.Fatalf("past time %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{
		"kind": "practice", "body": "ジョグから入ります", "venue": "駒沢競技場",
		"starts_at": starts.Format(time.RFC3339),
		"event":     "100m", "capacity": 3,
	})
	if rec.Code != 201 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Kind     string `json:"kind"`
		Practice struct {
			Venue      string `json:"venue"`
			Event      string `json:"event"`
			EventLabel string `json:"event_label"`
			Capacity   int    `json:"capacity"`
		} `json:"practice"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	wantTitle := "100m " + starts.Format("1月2日 15:04")
	if created.Kind != "practice" || created.Title != wantTitle || created.Practice.Venue != "駒沢競技場" || created.Practice.EventLabel != "100m" || created.Practice.Capacity != 3 {
		t.Fatalf("created %+v title %q", created, wantTitle)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{
		"title": "城南クラブ", "body": "初心者も歓迎です", "kind": "recruitment",
		"prefectures": []string{"13"}, "schedule": "毎週火曜 19:00-21:00",
	})
	if rec.Code != 201 {
		t.Fatalf("club %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{
		"title": "文章", "body": "レースの話",
	})
	if rec.Code != 201 {
		t.Fatalf("text %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/posts?sort=new&kind=recruitment", "", nil)
	if rec.Code != 200 {
		t.Fatalf("filter %d", rec.Code)
	}
	var filtered struct {
		Posts []struct {
			Kind     string `json:"kind"`
			Practice struct {
				Venue string `json:"venue"`
			} `json:"practice"`
		} `json:"posts"`
		Meta struct {
			Total int64 `json:"total"`
		} `json:"meta"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &filtered)
	if filtered.Meta.Total != 2 || len(filtered.Posts) != 2 {
		t.Fatalf("filtered %+v", filtered)
	}
	kinds := map[string]bool{}
	for _, post := range filtered.Posts {
		kinds[post.Kind] = true
		if post.Kind == "practice" && post.Practice.Venue != "駒沢競技場" {
			t.Fatalf("list practice %+v", post)
		}
	}
	if !kinds["practice"] || !kinds["recruitment"] {
		t.Fatalf("kinds %+v", kinds)
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/posts?sort=new&q="+url.QueryEscape("駒沢"), "", nil)
	var found struct {
		Meta struct {
			Total int64 `json:"total"`
		} `json:"meta"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &found)
	if found.Meta.Total != 1 {
		t.Fatalf("venue search %d", found.Meta.Total)
	}

	rec = doJSON(t, h, http.MethodPatch, "/v1/posts/"+created.ID, tb, map[string]any{
		"kind": "practice", "body": "だめ", "venue": "別の場所",
		"starts_at": starts.Format(time.RFC3339), "event": "200m", "capacity": 2,
	})
	if rec.Code != 403 {
		t.Fatalf("other edit %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPatch, "/v1/posts/"+created.ID, ta, map[string]any{
		"kind": "practice", "body": "流しをします", "venue": "織田フィールド",
		"starts_at": starts.Format(time.RFC3339), "event": "relay", "capacity": 2,
	})
	if rec.Code != 200 {
		t.Fatalf("update %d %s", rec.Code, rec.Body.String())
	}
	var updated struct {
		Title    string `json:"title"`
		Practice struct {
			Venue      string `json:"venue"`
			EventLabel string `json:"event_label"`
			Capacity   int    `json:"capacity"`
		} `json:"practice"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.Title != "リレー "+starts.Format("1月2日 15:04") || updated.Practice.Venue != "織田フィールド" || updated.Practice.Capacity != 2 {
		t.Fatalf("updated %+v", updated)
	}
}

func TestPracticeApplicationsFillAndClose(t *testing.T) {
	h := setup(t)
	hostID := uuid.New()
	guestID := uuid.New()
	otherID := uuid.New()
	host, guest, other := token(t, hostID), token(t, guestID), token(t, otherID)
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	starts := time.Now().In(loc).Add(72 * time.Hour)
	starts = time.Date(starts.Year(), starts.Month(), starts.Day(), 18, 0, 0, 0, loc)
	payload := map[string]any{
		"kind": "practice", "body": "ジョグから入ります", "venue": "〇〇競技場",
		"starts_at": starts.Format(time.RFC3339), "event": "100m", "capacity": 2,
	}
	rec := doJSON(t, h, http.MethodPost, "/v1/posts", host, payload)
	if rec.Code != 201 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/applications", host, nil)
	if rec.Code != 403 {
		t.Fatalf("host apply %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/applications", guest, nil)
	if rec.Code != 201 {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	var applied struct {
		Practice struct {
			MyApplication string `json:"my_application"`
		} `json:"practice"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &applied)
	if applied.Practice.MyApplication != "pending" {
		t.Fatalf("applied %+v", applied.Practice)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/applications", other, nil)
	if rec.Code != 201 {
		t.Fatalf("second apply %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/posts/"+created.ID, host, nil)
	var detail struct {
		Practice struct {
			Applications []struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"applications"`
		} `json:"practice"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &detail)
	if len(detail.Practice.Applications) != 2 {
		t.Fatalf("applications %+v", detail.Practice.Applications)
	}
	first := detail.Practice.Applications[0].ID
	second := detail.Practice.Applications[1].ID

	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/applications", token(t, uuid.New()), nil)
	if rec.Code != 201 {
		t.Fatalf("third apply %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/posts/"+created.ID, host, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &detail)
	if len(detail.Practice.Applications) != 3 {
		t.Fatalf("three applications %+v", detail.Practice.Applications)
	}
	third := detail.Practice.Applications[2].ID

	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/applications/"+first+"/accept", guest, nil)
	if rec.Code != 403 {
		t.Fatalf("guest accept %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/applications/"+first+"/accept", host, nil)
	if rec.Code != 200 {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/applications/"+second+"/accept", host, nil)
	if rec.Code != 200 {
		t.Fatalf("second accept %d %s", rec.Code, rec.Body.String())
	}
	var accepted struct {
		Practice struct {
			Status        string `json:"status"`
			AcceptedCount int    `json:"accepted_count"`
		} `json:"practice"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &accepted)
	if accepted.Practice.Status != "full" || accepted.Practice.AcceptedCount != 2 {
		t.Fatalf("full %+v", accepted.Practice)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/applications/"+third+"/accept", host, nil)
	if rec.Code != 409 {
		t.Fatalf("over capacity %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/applications", token(t, uuid.New()), nil)
	if rec.Code != 409 {
		t.Fatalf("apply when full %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/applications/"+third+"/decline", host, nil)
	if rec.Code != 200 {
		t.Fatalf("decline %d %s", rec.Code, rec.Body.String())
	}
	payload["capacity"] = 1
	rec = doJSON(t, h, http.MethodPatch, "/v1/posts/"+created.ID, host, payload)
	if rec.Code != 422 {
		t.Fatalf("shrink capacity %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/dissolve", guest, nil)
	if rec.Code != 403 {
		t.Fatalf("guest dissolve %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/dissolve", host, nil)
	if rec.Code != 200 {
		t.Fatalf("dissolve %d %s", rec.Code, rec.Body.String())
	}
	var dissolved struct {
		Practice struct {
			Status string `json:"status"`
		} `json:"practice"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &dissolved)
	if dissolved.Practice.Status != "dissolved" {
		t.Fatalf("dissolved %+v", dissolved.Practice)
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/applications", token(t, uuid.New()), nil)
	if rec.Code != 409 {
		t.Fatalf("apply after dissolve %d %s", rec.Code, rec.Body.String())
	}
}

func TestPracticeCommentsCloseWhenRecruitmentEnds(t *testing.T) {
	h := setup(t)
	hostID := uuid.New()
	guestID := uuid.New()
	strangerID := uuid.New()
	host, guest, stranger := token(t, hostID), token(t, guestID), token(t, strangerID)
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	starts := time.Now().In(loc).Add(72 * time.Hour)
	starts = time.Date(starts.Year(), starts.Month(), starts.Day(), 18, 0, 0, 0, loc)
	rec := doJSON(t, h, http.MethodPost, "/v1/posts", host, map[string]any{
		"kind": "practice", "body": "ジョグから入ります", "venue": "〇〇競技場",
		"starts_at": starts.Format(time.RFC3339), "event": "200m", "capacity": 1,
	})
	if rec.Code != 201 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/comments", stranger, map[string]string{"body": "場所を教えてください"})
	if rec.Code != 201 {
		t.Fatalf("comment while open %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/applications", guest, nil)
	if rec.Code != 201 {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	var applied struct {
		Practice struct {
			MyApplicationID string `json:"my_application_id"`
		} `json:"practice"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &applied)
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/applications/"+applied.Practice.MyApplicationID+"/accept", host, nil)
	if rec.Code != 200 {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/comments", stranger, map[string]string{"body": "まだ入れますか"})
	if rec.Code != 403 {
		t.Fatalf("stranger comment when full %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/comments", guest, map[string]string{"body": "少し遅れます"})
	if rec.Code != 201 {
		t.Fatalf("accepted comment when full %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/dissolve", host, nil)
	if rec.Code != 200 {
		t.Fatalf("dissolve %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/comments", stranger, map[string]string{"body": "解散後も書きます"})
	if rec.Code != 403 {
		t.Fatalf("stranger comment after dissolve %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/comments", host, map[string]string{"body": "ありがとうございました"})
	if rec.Code != 201 {
		t.Fatalf("host comment after dissolve %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/posts/"+created.ID+"/comments", "", nil)
	if rec.Code != 200 {
		t.Fatalf("list comments %d %s", rec.Code, rec.Body.String())
	}
	var listed struct {
		Comments []struct {
			Body string `json:"body"`
		} `json:"comments"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &listed)
	if listed.Meta.Total != 3 || len(listed.Comments) != 3 {
		t.Fatalf("comments stayed readable %+v", listed)
	}
}

func TestPracticeRatingsFollowTheNextPost(t *testing.T) {
	h := setup(t)
	hostID := uuid.New()
	guestID := uuid.New()
	strangerID := uuid.New()
	host, guest, stranger := token(t, hostID), token(t, guestID), token(t, strangerID)
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	starts := time.Now().In(loc).Add(72 * time.Hour)
	starts = time.Date(starts.Year(), starts.Month(), starts.Day(), 18, 0, 0, 0, loc)
	rec := doJSON(t, h, http.MethodPost, "/v1/posts", host, map[string]any{
		"kind": "practice", "body": "流しをします", "venue": "〇〇競技場",
		"starts_at": starts.Format(time.RFC3339), "event": "200m", "capacity": 1,
	})
	if rec.Code != 201 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/applications", guest, nil)
	if rec.Code != 201 {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	var applied struct {
		Practice struct {
			MyApplicationID string `json:"my_application_id"`
		} `json:"practice"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &applied)
	applicationID := applied.Practice.MyApplicationID
	if applicationID == "" {
		t.Fatal("missing application id")
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/posts/"+created.ID, host, nil)
	var hostView struct {
		Practice struct {
			Applications []struct {
				ID string `json:"id"`
			} `json:"applications"`
		} `json:"practice"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &hostView)
	if len(hostView.Practice.Applications) != 1 {
		t.Fatalf("applications %+v", hostView.Practice.Applications)
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/applications/"+applicationID+"/accept", host, nil)
	if rec.Code != 200 {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}

	rating := map[string]any{"score": "good", "comment": "時間通りでした"}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/applications/"+applicationID+"/rating", guest, rating)
	if rec.Code != 409 {
		t.Fatalf("rate before dissolve %d %s", rec.Code, rec.Body.String())
	}
	practiceHeld(t, created.ID)
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/dissolve", host, nil)
	if rec.Code != 200 {
		t.Fatalf("dissolve %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/applications/"+applicationID+"/rating", stranger, rating)
	if rec.Code != 403 {
		t.Fatalf("stranger rate %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/applications/"+applicationID+"/rating", guest, map[string]any{"score": "great"})
	if rec.Code != 422 {
		t.Fatalf("bad score %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/applications/"+applicationID+"/rating", guest, rating)
	if rec.Code != 201 {
		t.Fatalf("guest rate %d %s", rec.Code, rec.Body.String())
	}
	var guestRated struct {
		Practice struct {
			RatedHost bool `json:"rated_host"`
		} `json:"practice"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &guestRated)
	if !guestRated.Practice.RatedHost {
		t.Fatal("guest rating was not recorded")
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/applications/"+applicationID+"/rating", guest, rating)
	if rec.Code != 409 {
		t.Fatalf("second rate %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/applications/"+applicationID+"/rating", host, map[string]any{"score": "ok", "comment": ""})
	if rec.Code != 200 && rec.Code != 201 {
		t.Fatalf("host rate %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/me/practice-reputation", host, nil)
	var mine struct {
		Good     int `json:"good"`
		Ok       int `json:"ok"`
		Bad      int `json:"bad"`
		Comments []struct {
			Comment string `json:"comment"`
		} `json:"comments"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &mine)
	if rec.Code != 200 || mine.Good != 1 || mine.Ok != 0 || len(mine.Comments) != 1 || mine.Comments[0].Comment != "時間通りでした" {
		t.Fatalf("reputation %d %+v", rec.Code, mine)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts", host, map[string]any{
		"kind": "practice", "body": "次の練習です", "venue": "別の競技場",
		"starts_at": starts.Add(24 * time.Hour).Format(time.RFC3339), "event": "400m", "capacity": 2,
	})
	if rec.Code != 201 {
		t.Fatalf("next post %d %s", rec.Code, rec.Body.String())
	}
	var next struct {
		Practice struct {
			Reputation struct {
				Good     int `json:"good"`
				Comments []struct {
					Comment string `json:"comment"`
				} `json:"comments"`
			} `json:"reputation"`
		} `json:"practice"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &next)
	if next.Practice.Reputation.Good != 1 || len(next.Practice.Reputation.Comments) != 1 {
		t.Fatalf("next reputation %+v", next.Practice.Reputation)
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/posts?sort=new&kind=recruitment", "", nil)
	var listed struct {
		Posts []struct {
			Kind     string `json:"kind"`
			Practice struct {
				Reputation struct {
					Good int `json:"good"`
				} `json:"reputation"`
			} `json:"practice"`
		} `json:"posts"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &listed)
	var found bool
	for _, post := range listed.Posts {
		if post.Kind == "practice" && post.Practice.Reputation.Good == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("list reputation %+v", listed.Posts)
	}
}

func setAdmin(t *testing.T, id uuid.UUID) {
	t.Helper()
	dsn := testDSN(t)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tag, err := pool.Exec(ctx, `UPDATE profiles SET is_admin = true WHERE id = $1`, id)
	if err != nil {
		t.Fatal(err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatalf("admin rows %d", tag.RowsAffected())
	}
}

type noteItem struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	PostID    string `json:"post_id"`
	PostTitle string `json:"post_title"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Read      bool   `json:"read"`
}

func listNotifications(t *testing.T, h http.Handler, tok string) ([]noteItem, int64) {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/me/notifications", tok, nil)
	if rec.Code != 200 {
		t.Fatalf("notifications %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Notifications []noteItem `json:"notifications"`
		UnreadCount   int64      `json:"unread_count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Notifications, body.UnreadCount
}

func notesFor(items []noteItem, postID, kind string) []noteItem {
	var out []noteItem
	for _, item := range items {
		if item.PostID == postID && (kind == "" || item.Kind == kind) {
			out = append(out, item)
		}
	}
	return out
}

func TestNotifications(t *testing.T) {
	h := setup(t)
	authorID := uuid.New()
	viewerID := uuid.New()
	lateID := uuid.New()
	declinedID := uuid.New()
	author := token(t, authorID)
	viewer := token(t, viewerID)
	late := token(t, lateID)
	declined := token(t, declinedID)

	rec := doJSON(t, h, http.MethodGet, "/v1/me/notifications", "", nil)
	if rec.Code != 401 {
		t.Fatalf("anonymous %d", rec.Code)
	}
	if rec = doJSON(t, h, http.MethodGet, "/v1/me", author, nil); rec.Code != 200 {
		t.Fatalf("author profile %d %s", rec.Code, rec.Body.String())
	}
	if rec = doJSON(t, h, http.MethodGet, "/v1/me", viewer, nil); rec.Code != 200 {
		t.Fatalf("viewer profile %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts", author, map[string]any{
		"title": "レース感想", "body": "楽しかったです",
	})
	if rec.Code != 201 {
		t.Fatalf("text %d %s", rec.Code, rec.Body.String())
	}
	var textPost struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &textPost)

	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	starts := time.Now().In(loc).Add(72 * time.Hour)
	starts = time.Date(starts.Year(), starts.Month(), starts.Day(), 18, 0, 0, 0, loc)
	practiceBody := map[string]any{
		"kind": "practice", "body": "ジョグから入ります", "venue": "〇〇競技場",
		"starts_at": starts.Format(time.RFC3339), "event": "100m", "capacity": 2,
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts", author, practiceBody)
	if rec.Code != 201 {
		t.Fatalf("practice %d %s", rec.Code, rec.Body.String())
	}
	var practicePost struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &practicePost)

	if items, unread := listNotifications(t, h, viewer); len(items) != 0 || unread != 0 {
		t.Fatalf("practice and text should not notify, items=%+v unread=%d", items, unread)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts", author, map[string]any{
		"title": "城南クラブ", "body": "初心者も歓迎です", "kind": "recruitment",
		"prefectures": []string{"13"}, "venue": "駒沢競技場",
		"schedule": "毎週火曜 19:00-21:00", "gender_ratio": "男性7：女性3",
		"average_age": "25歳", "recruit_age": "18〜30歳",
	})
	if rec.Code != 201 {
		t.Fatalf("recruitment %d %s", rec.Code, rec.Body.String())
	}
	var recruitmentPost struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &recruitmentPost)

	authorNotes, _ := listNotifications(t, h, author)
	if len(notesFor(authorNotes, recruitmentPost.ID, "recruitment_post")) != 0 {
		t.Fatalf("author was notified of their own recruitment %+v", authorNotes)
	}
	viewerNotes, viewerUnread := listNotifications(t, h, viewer)
	recruitmentNotes := notesFor(viewerNotes, recruitmentPost.ID, "recruitment_post")
	if len(recruitmentNotes) != 1 || recruitmentNotes[0].PostTitle != "城南クラブ" || recruitmentNotes[0].Read || viewerUnread != 1 {
		t.Fatalf("viewer recruitment %+v unread %d", viewerNotes, viewerUnread)
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/posts?sort=new&kind=recruitment", "", nil)
	if rec.Code != 200 {
		t.Fatalf("recruitment list %d %s", rec.Code, rec.Body.String())
	}
	var recruitmentFeed struct {
		Posts []struct {
			ID string `json:"id"`
		} `json:"posts"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &recruitmentFeed)
	var listed bool
	for _, post := range recruitmentFeed.Posts {
		if post.ID == recruitmentPost.ID {
			listed = true
		}
	}
	if !listed {
		t.Fatalf("recruitment missing from feed %+v", recruitmentFeed.Posts)
	}
	if len(notesFor(viewerNotes, practicePost.ID, "")) != 0 || len(notesFor(viewerNotes, textPost.ID, "")) != 0 {
		t.Fatalf("unexpected post notifications %+v", viewerNotes)
	}

	if rec = doJSON(t, h, http.MethodGet, "/v1/me", late, nil); rec.Code != 200 {
		t.Fatalf("late profile %d %s", rec.Code, rec.Body.String())
	}
	if items, _ := listNotifications(t, h, late); len(items) != 0 {
		t.Fatalf("late user received past notifications %+v", items)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+textPost.ID+"/comments", viewer, map[string]any{"body": "同感です"})
	if rec.Code != 201 {
		t.Fatalf("comment %d %s", rec.Code, rec.Body.String())
	}
	var comment struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &comment)
	authorNotes, authorUnread := listNotifications(t, h, author)
	commentNotes := notesFor(authorNotes, textPost.ID, "comment")
	if len(commentNotes) != 1 || commentNotes[0].PostTitle != "レース感想" || authorUnread != 1 {
		t.Fatalf("comment notification %+v unread %d", authorNotes, authorUnread)
	}
	if viewerNotes, _ = listNotifications(t, h, viewer); len(notesFor(viewerNotes, textPost.ID, "comment")) != 0 {
		t.Fatalf("commenter was notified %+v", viewerNotes)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+textPost.ID+"/comments", author, map[string]any{"body": "ありがとうございます"})
	if rec.Code != 201 {
		t.Fatalf("own comment %d %s", rec.Code, rec.Body.String())
	}
	authorNotes, _ = listNotifications(t, h, author)
	if len(notesFor(authorNotes, textPost.ID, "comment")) != 1 {
		t.Fatalf("own comment created a notification %+v", authorNotes)
	}

	rec = doJSON(t, h, http.MethodDelete, "/v1/comments/"+comment.ID, viewer, nil)
	if rec.Code != 204 {
		t.Fatalf("delete comment %d %s", rec.Code, rec.Body.String())
	}
	authorNotes, authorUnread = listNotifications(t, h, author)
	if len(notesFor(authorNotes, textPost.ID, "comment")) != 0 || authorUnread != 0 {
		t.Fatalf("deleted comment still listed %+v unread %d", authorNotes, authorUnread)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+textPost.ID+"/comments", viewer, map[string]any{"body": "もう一度"})
	if rec.Code != 201 {
		t.Fatalf("second comment %d %s", rec.Code, rec.Body.String())
	}
	authorNotes, _ = listNotifications(t, h, author)
	commentNotes = notesFor(authorNotes, textPost.ID, "comment")
	if len(commentNotes) != 1 {
		t.Fatalf("second comment %+v", authorNotes)
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/me/notifications/"+commentNotes[0].ID+"/read", viewer, nil)
	if rec.Code != 404 {
		t.Fatalf("other user mark read %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/me/notifications/"+commentNotes[0].ID+"/read", author, nil)
	if rec.Code != 204 {
		t.Fatalf("mark read %d %s", rec.Code, rec.Body.String())
	}
	authorNotes, authorUnread = listNotifications(t, h, author)
	commentNotes = notesFor(authorNotes, textPost.ID, "comment")
	if len(commentNotes) != 1 || !commentNotes[0].Read || authorUnread != 0 {
		t.Fatalf("after read %+v unread %d", authorNotes, authorUnread)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+practicePost.ID+"/practice/applications", viewer, nil)
	if rec.Code != 201 {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	var viewerApplication struct {
		Practice struct {
			MyApplicationID string `json:"my_application_id"`
		} `json:"practice"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &viewerApplication)
	authorNotes, _ = listNotifications(t, h, author)
	if len(notesFor(authorNotes, practicePost.ID, "practice_application")) != 1 {
		t.Fatalf("application notification %+v", authorNotes)
	}
	viewerNotes, _ = listNotifications(t, h, viewer)
	if len(notesFor(viewerNotes, practicePost.ID, "practice_application")) != 0 {
		t.Fatalf("applicant notified themselves %+v", viewerNotes)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+practicePost.ID+"/practice/applications", declined, nil)
	if rec.Code != 201 {
		t.Fatalf("second apply %d %s", rec.Code, rec.Body.String())
	}
	var declinedApplication struct {
		Practice struct {
			MyApplicationID string `json:"my_application_id"`
		} `json:"practice"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &declinedApplication)
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+practicePost.ID+"/practice/applications/"+viewerApplication.Practice.MyApplicationID+"/accept", author, nil)
	if rec.Code != 200 {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+practicePost.ID+"/practice/applications/"+declinedApplication.Practice.MyApplicationID+"/decline", author, nil)
	if rec.Code != 200 {
		t.Fatalf("decline %d %s", rec.Code, rec.Body.String())
	}
	viewerNotes, _ = listNotifications(t, h, viewer)
	if len(notesFor(viewerNotes, practicePost.ID, "practice_accepted")) != 1 {
		t.Fatalf("accepted notification %+v", viewerNotes)
	}
	declinedNotes, _ := listNotifications(t, h, declined)
	if len(notesFor(declinedNotes, practicePost.ID, "practice_accepted")) != 0 {
		t.Fatalf("decline notified %+v", declinedNotes)
	}

	emptyPractice := practiceBody
	emptyPractice["event"] = "200m"
	rec = doJSON(t, h, http.MethodPost, "/v1/posts", author, emptyPractice)
	if rec.Code != 201 {
		t.Fatalf("empty practice %d %s", rec.Code, rec.Body.String())
	}
	var emptyPost struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &emptyPost)
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+emptyPost.ID+"/practice/dissolve", author, nil)
	if rec.Code != 200 {
		t.Fatalf("empty dissolve %d %s", rec.Code, rec.Body.String())
	}
	authorNotes, _ = listNotifications(t, h, author)
	if len(notesFor(authorNotes, emptyPost.ID, "practice_rate")) != 0 {
		t.Fatalf("rate notification without participants %+v", authorNotes)
	}

	practiceHeld(t, practicePost.ID)
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+practicePost.ID+"/practice/dissolve", author, nil)
	if rec.Code != 200 {
		t.Fatalf("dissolve %d %s", rec.Code, rec.Body.String())
	}
	authorNotes, authorUnread = listNotifications(t, h, author)
	if len(notesFor(authorNotes, practicePost.ID, "practice_rate")) != 1 || authorUnread < 1 {
		t.Fatalf("host rate notification %+v unread %d", authorNotes, authorUnread)
	}
	viewerNotes, _ = listNotifications(t, h, viewer)
	if len(notesFor(viewerNotes, practicePost.ID, "practice_rate")) != 1 {
		t.Fatalf("guest rate notification %+v", viewerNotes)
	}
	declinedNotes, _ = listNotifications(t, h, declined)
	if len(notesFor(declinedNotes, practicePost.ID, "practice_rate")) != 0 {
		t.Fatalf("declined user got a rate notification %+v", declinedNotes)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/me/notifications/read", author, nil)
	if rec.Code != 204 {
		t.Fatalf("mark all %d %s", rec.Code, rec.Body.String())
	}
	if _, authorUnread = listNotifications(t, h, author); authorUnread != 0 {
		t.Fatalf("unread after mark all %d", authorUnread)
	}
}

// testDSN は統合テストの接続先を返す。本番 DB を壊さないよう、ローカル以外は拒否する。
func testDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://tracktalk:tracktalk@127.0.0.1:55432/tracktalk?sslmode=disable"
	}
	if err := config.RequireLocalDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	return dsn
}

// 練習の評価のひとことは誰でも読めるので、連絡先を書けず、通報されたら運営が非表示にできる。
func TestPracticeRatingCommentCanBeReportedAndHidden(t *testing.T) {
	h := setup(t)
	host, guest := token(t, uuid.New()), token(t, uuid.New())
	starts := time.Now().Add(72 * time.Hour).Format(time.RFC3339)
	rec := doJSON(t, h, http.MethodPost, "/v1/posts", host, map[string]any{
		"kind": "practice", "body": "ジョグから入ります", "venue": "〇〇競技場", "starts_at": starts, "event": "100m", "capacity": 2,
	})
	if rec.Code != 201 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	base := "/v1/posts/" + created.ID + "/practice"
	if rec := doJSON(t, h, http.MethodPost, base+"/applications", guest, nil); rec.Code != 201 {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	var detail struct {
		Practice struct {
			Applications []struct {
				ID string `json:"id"`
			} `json:"applications"`
		} `json:"practice"`
	}
	_ = json.Unmarshal(doJSON(t, h, http.MethodGet, "/v1/posts/"+created.ID, host, nil).Body.Bytes(), &detail)
	app := detail.Practice.Applications[0].ID
	if rec := doJSON(t, h, http.MethodPost, base+"/applications/"+app+"/accept", host, nil); rec.Code != 200 {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}
	practiceHeld(t, created.ID)
	if rec := doJSON(t, h, http.MethodPost, base+"/dissolve", host, nil); rec.Code != 200 {
		t.Fatalf("dissolve %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, base+"/applications/"+app+"/rating", guest, map[string]any{"score": "bad", "comment": "インスタは @someone_123 です"})
	if rec.Code != 422 || !strings.Contains(rec.Body.String(), "comment") {
		t.Fatalf("contact in comment %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, base+"/applications/"+app+"/rating", guest, map[string]any{"score": "bad", "comment": "ひどい人でした"}); rec.Code != 200 && rec.Code != 201 {
		t.Fatalf("rate %d %s", rec.Code, rec.Body.String())
	}

	type reputation struct {
		Practice struct {
			Reputation struct {
				Comments []struct {
					ID      string `json:"id"`
					Comment string `json:"comment"`
				} `json:"comments"`
			} `json:"reputation"`
		} `json:"practice"`
	}
	var public reputation
	_ = json.Unmarshal(doJSON(t, h, http.MethodGet, "/v1/posts/"+created.ID, "", nil).Body.Bytes(), &public)
	if len(public.Practice.Reputation.Comments) != 1 || public.Practice.Reputation.Comments[0].ID == "" {
		t.Fatalf("public comments %+v", public.Practice.Reputation)
	}
	ratingID := public.Practice.Reputation.Comments[0].ID

	report := map[string]any{"target_type": "practice_rating", "target_id": ratingID, "reason": "harassment", "detail": ""}
	if rec := doJSON(t, h, http.MethodPost, "/v1/reports", host, report); rec.Code != http.StatusCreated {
		t.Fatalf("report %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/reports", host, map[string]any{"target_type": "practice_rating", "target_id": uuid.NewString(), "reason": "other"}); rec.Code != http.StatusNotFound {
		t.Fatalf("report unknown rating %d", rec.Code)
	}

	admin := adminToken(t, h)
	rec = doJSON(t, h, http.MethodGet, "/v1/admin/reports", admin, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "ひどい人でした") {
		t.Fatalf("admin reports %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/practice-ratings/"+ratingID+"/hide", host, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin hide %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/practice-ratings/"+ratingID+"/hide", admin, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("hide %d %s", rec.Code, rec.Body.String())
	}
	var after reputation
	_ = json.Unmarshal(doJSON(t, h, http.MethodGet, "/v1/posts/"+created.ID, "", nil).Body.Bytes(), &after)
	if len(after.Practice.Reputation.Comments) != 0 {
		t.Fatalf("hidden comment still shown %+v", after.Practice.Reputation)
	}
}

// 特商法の表記で省略した住所・電話番号の開示請求は、お問い合わせの種類として受け付ける。
func TestInquiryAcceptsCommerceDisclosureRequest(t *testing.T) {
	h := setup(t)
	ta := token(t, uuid.New())
	rec := doJSON(t, h, http.MethodPost, "/v1/me/inquiries", ta, map[string]string{
		"category": "disclosure", "subject": "所在地の開示", "body": "特定商取引法の表記の所在地と電話番号を教えてください", "email": "buyer@example.com",
	})
	if rec.Code != 201 || !strings.Contains(rec.Body.String(), `"category":"disclosure"`) {
		t.Fatalf("disclosure %d %s", rec.Code, rec.Body.String())
	}
}
