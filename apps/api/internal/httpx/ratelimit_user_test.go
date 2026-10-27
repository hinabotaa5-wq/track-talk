package httpx

import (
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/hinata/track-talk/apps/api/internal/auth"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

func TestRateLimiterCountsWritesPerUserOnSharedIP(t *testing.T) {
	secret := "test-jwt-secret-for-ci-only"
	supabaseURL := "https://example.invalid"
	v, err := auth.NewVerifier(secret, supabaseURL, "authenticated")
	if err != nil {
		t.Fatal(err)
	}
	token := func(id string) string {
		t.Helper()
		s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": id,
			"aud": "authenticated",
			"iss": supabaseURL + "/auth/v1",
			"exp": time.Now().Add(time.Hour).Unix(),
		}).SignedString([]byte(secret))
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	h := NewRateLimiter(v).Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	post := func(bearer string) int {
		req := httptest.NewRequest(http.MethodPost, "/v1/posts/x/like", nil)
		req.RemoteAddr = "203.0.113.7:1234"
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	a := token("11111111-1111-4111-8111-111111111111")
	b := token("22222222-2222-4222-8222-222222222222")
	for i := 0; i < 30; i++ {
		if code := post(a); code != http.StatusNoContent {
			t.Fatalf("user a write %d: %d", i, code)
		}
	}
	if code := post(a); code != http.StatusTooManyRequests {
		t.Fatalf("user a over limit: %d", code)
	}
	// 同じ IP の別の人と、ログインしていない人は、a の回数に巻き込まれない。
	if code := post(b); code != http.StatusNoContent {
		t.Fatalf("user b on same ip: %d", code)
	}
	if code := post(""); code != http.StatusNoContent {
		t.Fatalf("anonymous on same ip: %d", code)
	}
	// 壊れたトークンは IP で数える。
	for i := 0; i < 29; i++ {
		post("broken")
	}
	if code := post("broken"); code != http.StatusTooManyRequests {
		t.Fatalf("broken token over ip limit: %d", code)
	}
}

func TestServerCompressesJSON(t *testing.T) {
	v, err := auth.NewVerifier("test-jwt-secret-for-ci-only", "", "authenticated")
	if err != nil {
		t.Fatal(err)
	}
	// 通知の鍵が無いと 503 の JSON が返る。DB を使わずに JSON の応答を受け取れる。
	h := New(config.Config{}, nil, v)
	req := httptest.NewRequest(http.MethodGet, "/v1/push/public-key", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("status %d encoding %q", rec.Code, rec.Header().Get("Content-Encoding"))
	}
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.NewDecoder(zr).Decode(&body); err != nil || body["error"] == nil {
		t.Fatalf("body %v err %v", body, err)
	}
	req = httptest.NewRequest(http.MethodGet, "/v1/push/public-key", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatalf("compressed without Accept-Encoding: %q", rec.Header().Get("Content-Encoding"))
	}
}
