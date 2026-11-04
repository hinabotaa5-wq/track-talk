package httpx

import (
	"bytes"
	"encoding/json"
	"log/slog"
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
	h := RequireAuth(v, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestLogPathHidesGuardianToken(t *testing.T) {
	token := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
	for in, want := range map[string]string{
		"/v1/guardian/" + token:                           "/v1/guardian/:token",
		"/v1/guardian/" + token + "/contracts/x/messages": "/v1/guardian/:token/contracts/x/messages",
		"/v1/posts/123":                                   "/v1/posts/123",
		"/health":                                         "/health",
	} {
		if got := logPath(in); got != want {
			t.Fatalf("logPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRequestLogRecordsAuthenticatedUser(t *testing.T) {
	secret := "test-jwt-secret-for-ci-only"
	supabaseURL := "https://example.invalid"
	v, err := auth.NewVerifier(secret, supabaseURL, "authenticated")
	if err != nil {
		t.Fatal(err)
	}
	id := "11111111-1111-4111-8111-111111111111"
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": id,
		"aud": "authenticated",
		"iss": supabaseURL + "/auth/v1",
		"exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	optional := OptionalAuth(v, nil)
	required := RequireAuth(v, nil)

	cases := []struct {
		name    string
		handler http.Handler
		header  string
		status  int
		userID  string
	}{
		{"optional with token", optional(ok), "Bearer " + tok, http.StatusNoContent, id},
		{"required with token", optional(required(ok)), "Bearer " + tok, http.StatusNoContent, id},
		{"optional without token", optional(ok), "", http.StatusNoContent, ""},
		{"optional with bad token", optional(ok), "Bearer bad", http.StatusNoContent, ""},
		{"required without token", optional(required(ok)), "", http.StatusUnauthorized, ""},
		{"required with bad token", optional(required(ok)), "Bearer bad", http.StatusUnauthorized, ""},
		{"no auth", ok, "Bearer " + tok, http.StatusNoContent, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			previous := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
			t.Cleanup(func() { slog.SetDefault(previous) })

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			RequestLog(tc.handler).ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status %d want %d", rec.Code, tc.status)
			}

			// 認証の失敗も slog に出るので、リクエストの行だけを見る。
			var found bool
			for _, line := range bytes.Split(bytes.TrimSpace(logs.Bytes()), []byte("\n")) {
				var entry struct {
					Msg    string  `json:"msg"`
					UserID *string `json:"user_id"`
				}
				if err := json.Unmarshal(line, &entry); err != nil {
					t.Fatalf("parse %q: %v", line, err)
				}
				if entry.Msg != "http" {
					continue
				}
				found = true
				if entry.UserID == nil {
					t.Fatalf("user_id missing: %s", line)
				}
				if *entry.UserID != tc.userID {
					t.Fatalf("user_id %q want %q", *entry.UserID, tc.userID)
				}
			}
			if !found {
				t.Fatalf("no request log: %s", logs.String())
			}
		})
	}
}
