package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/hinata/track-talk/apps/api/internal/auth"
)

func TestRequireAuthRejectsMissingExpAndWrongIssuer(t *testing.T) {
	secret := "test-jwt-secret-for-ci-only"
	supabaseURL := "https://example.invalid"
	v, err := auth.NewVerifier(secret, supabaseURL, "authenticated")
	if err != nil {
		t.Fatal(err)
	}
	h := RequireAuth(v)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	id := "11111111-1111-4111-8111-111111111111"
	sign := func(claims jwt.MapClaims) string {
		t.Helper()
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		s, err := tok.SignedString([]byte(secret))
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	valid := jwt.MapClaims{
		"sub": id,
		"aud": "authenticated",
		"iss": supabaseURL + "/auth/v1",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+sign(valid))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("valid %d %s", rec.Code, rec.Body.String())
	}
	delete(valid, "exp")
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+sign(valid))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing exp %d", rec.Code)
	}
	valid["exp"] = time.Now().Add(time.Hour).Unix()
	valid["iss"] = "https://evil.example/auth/v1"
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+sign(valid))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong issuer %d", rec.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	h := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d", rec.Code)
	}
	if got := rec.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("referrer %q", got)
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("frame %q", got)
	}
	if !strings.Contains(rec.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Fatalf("csp %q", rec.Header().Get("Content-Security-Policy"))
	}
}

func TestClientIPIgnoresSpoofedForwardedFor(t *testing.T) {
	first := httptest.NewRequest(http.MethodPost, "/v1/posts", nil)
	first.RemoteAddr = "203.0.113.10:54321"
	first.Header.Set("X-Forwarded-For", "198.51.100.1")

	second := httptest.NewRequest(http.MethodPost, "/v1/posts", nil)
	second.RemoteAddr = "203.0.113.10:54321"
	second.Header.Set("X-Forwarded-For", "198.51.100.99")

	if got, want := clientIP(first), "203.0.113.10"; got != want {
		t.Fatalf("first %s want %s", got, want)
	}
	if clientIP(first) != clientIP(second) {
		t.Fatalf("spoofed header changed the key: %s vs %s", clientIP(first), clientIP(second))
	}
}

func TestClientIPUsesAddressFromTrustedProxy(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/posts", nil)
	req.RemoteAddr = "172.18.0.3:54321"
	req.Header.Set("X-Forwarded-For", "198.51.100.1, 203.0.113.50")
	if got := clientIP(req); got != "203.0.113.50" {
		t.Fatalf("appended %s", got)
	}
	req.Header.Set("X-Forwarded-For", "203.0.113.50")
	if got := clientIP(req); got != "203.0.113.50" {
		t.Fatalf("single %s", got)
	}
}
