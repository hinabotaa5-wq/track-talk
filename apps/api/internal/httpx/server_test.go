package httpx_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/auth"
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
	_, _ = pool.Exec(ctx, `TRUNCATE reports, post_likes, comments, posts, profiles CASCADE`)
	v, err := auth.NewVerifier(secret, "", "authenticated")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{CORSOrigins: []string{"http://localhost:3000"}, JWTSecret: secret, JWTAUD: "authenticated", DatabaseURL: dsn}
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

	rec := doJSON(t, h, http.MethodPut, "/v1/me/profile", ta, map[string]string{"nickname": "runnerA", "bio": "100m"})
	if rec.Code != 200 {
		t.Fatalf("profile a %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPut, "/v1/me/profile", tb, map[string]string{"nickname": "runnerB", "bio": ""})
	if rec.Code != 200 {
		t.Fatalf("profile b %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{
		"title": "日本選手権 100m", "body": "決勝の展開が熱かった", "category_id": "race_impression",
	})
	if rec.Code != 201 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	rec = doJSON(t, h, http.MethodPatch, "/v1/posts/"+created.ID, tb, map[string]any{
		"title": "改ざん", "body": "だめ", "category_id": "general",
	})
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

func TestValidation(t *testing.T) {
	h := setup(t)
	a := uuid.New()
	ta := token(t, a)
	doJSON(t, h, http.MethodPut, "/v1/me/profile", ta, map[string]string{"nickname": "okname", "bio": ""})
	rec := doJSON(t, h, http.MethodPost, "/v1/posts", ta, map[string]any{"title": "", "body": "", "category_id": "nope"})
	if rec.Code != 422 {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
}
