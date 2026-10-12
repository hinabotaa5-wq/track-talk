package httpx_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
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
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://tracktalk:tracktalk@localhost:5432/tracktalk?sslmode=disable"
	}
	secret := os.Getenv("SUPABASE_JWT_SECRET")
	if secret == "" {
		secret = "test-jwt-secret-for-ci-only"
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
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
	_, _ = pool.Exec(ctx, `TRUNCATE inquiries, reports, post_saves, post_likes, comments, practice_applications, posts, profiles CASCADE`)
	v, err := auth.NewVerifier(secret, "", "authenticated")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		CORSOrigins:    []string{"http://localhost:3000"},
		JWTSecret:      secret,
		JWTAUD:         "authenticated",
		DatabaseURL:    dsn,
		CloudName:      os.Getenv("CLOUDINARY_CLOUD_NAME"),
		CloudAPIKey:    os.Getenv("CLOUDINARY_API_KEY"),
		CloudAPISecret: os.Getenv("CLOUDINARY_API_SECRET"),
		CloudAPIBase:   os.Getenv("CLOUDINARY_API_BASE"),
	}
	return httpx.New(cfg, pool, v)
}

func token(t *testing.T, id uuid.UUID) string {
	t.Helper()
	secret := os.Getenv("SUPABASE_JWT_SECRET")
	if secret == "" {
		secret = "test-jwt-secret-for-ci-only"
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": id.String(),
		"aud": "authenticated",
		"exp": time.Now().Add(time.Hour).Unix(),
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
		"title": "", "body": ""})
	if rec.Code != 201 {
		t.Fatalf("empty text %d %s", rec.Code, rec.Body.String())
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

func TestSignedVideoJoinsTheList(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"resource_type":"video"}`))
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
		CloudName string `json:"cloud_name"`
		APIKey    string `json:"api_key"`
		PublicID  string `json:"public_id"`
		Signature string `json:"signature"`
		Timestamp int64  `json:"timestamp"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &signed)
	if signed.CloudName != "demo" || signed.APIKey != "key" || !cloudinary.ValidPublicID(signed.PublicID) {
		t.Fatalf("signed %+v", signed)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("secret")) {
		t.Fatal("secret leaked")
	}
	want := cloudinary.Sign(map[string]string{
		"public_id": signed.PublicID,
		"timestamp": strconv.FormatInt(signed.Timestamp, 10),
	}, "secret")
	if signed.Signature != want {
		t.Fatalf("signature %s want %s", signed.Signature, want)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{
		"title": "", "body": "", "kind": "video",
		"video_public_id": signed.PublicID,
	})
	if rec.Code != 201 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Kind         string `json:"kind"`
		ThumbnailURL string `json:"thumbnail_url"`
		PlaybackURL  string `json:"playback_url"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.Kind != "video" || created.ThumbnailURL == "" || created.PlaybackURL == "" {
		t.Fatalf("created %+v", created)
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
		_, _ = w.Write([]byte(`{"resource_type":"image"}`))
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
		"title": "スタート", "body": "写真つき", "image_public_id": signed.PublicID,
	})
	if rec.Code != 201 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Kind          string `json:"kind"`
		ThumbnailURL  string `json:"thumbnail_url"`
		PlaybackURL   string `json:"playback_url"`
		ImagePublicID string `json:"image_public_id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.Kind != "text" || created.ImagePublicID != signed.PublicID || created.PlaybackURL != "" || !strings.Contains(created.ThumbnailURL, "/image/upload/") {
		t.Fatalf("created %+v", created)
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
		"url": "https://youtu.be/abcdefghijk"})
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
	if rec.Code != 201 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/me/inquiries", ta, nil)
	if rec.Code != 200 {
		t.Fatalf("mine %d %s", rec.Code, rec.Body.String())
	}
	var list struct {
		Inquiries []struct {
			Subject string `json:"subject"`
			UserID  string `json:"user_id"`
		} `json:"inquiries"`
		Meta struct {
			Total int64 `json:"total"`
		} `json:"meta"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if list.Meta.Total != 1 || len(list.Inquiries) != 1 || list.Inquiries[0].Subject != "ログインできない" || list.Inquiries[0].UserID != a.String() {
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
	if list.Meta.Total != 1 || len(list.Inquiries) != 1 || list.Inquiries[0].UserID != a.String() {
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
		_, _ = w.Write([]byte(`{"resource_type":"image"}`))
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
		"starts_at": starts.Format(time.RFC3339), "event": "other", "event_other": "ハードル", "capacity": 2,
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
	if updated.Title != "ハードル "+starts.Format("1月2日 15:04") || updated.Practice.Venue != "織田フィールド" || updated.Practice.Capacity != 2 {
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

func setAdmin(t *testing.T, id uuid.UUID) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://tracktalk:tracktalk@localhost:5432/tracktalk?sslmode=disable"
	}
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
