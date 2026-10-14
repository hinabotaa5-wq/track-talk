package auth

import (
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

func TestParseHS256(t *testing.T) {
	secret := "test-jwt-secret-for-ci-only"
	supabaseURL := "https://example.invalid"
	v, err := NewVerifier(secret, supabaseURL, "authenticated")
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
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
		"sub": id.String(),
		"aud": "authenticated",
		"iss": supabaseURL + "/auth/v1",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	got, err := v.Parse(sign(valid))
	if err != nil {
		t.Fatal(err)
	}
	if got != id {
		t.Fatalf("got %s", got)
	}
	if _, err := v.Parse("abc"); err == nil {
		t.Fatal("expected error")
	}
	noExp := jwt.MapClaims{
		"sub": id.String(),
		"aud": "authenticated",
		"iss": supabaseURL + "/auth/v1",
	}
	if _, err := v.Parse(sign(noExp)); err == nil {
		t.Fatal("missing exp")
	}
	wrongIss := jwt.MapClaims{
		"sub": id.String(),
		"aud": "authenticated",
		"iss": "https://evil.example/auth/v1",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	if _, err := v.Parse(sign(wrongIss)); err == nil {
		t.Fatal("wrong issuer")
	}
}

func TestParseAcceptsLiveSupabaseIssuer(t *testing.T) {
	_ = godotenv.Load("../../../../.env")
	supabaseURL := os.Getenv("SUPABASE_URL")
	if supabaseURL == "" {
		t.Skip("SUPABASE_URL is unset")
	}
	secret := "test-jwt-secret-for-ci-only"
	v, err := NewVerifier(secret, supabaseURL, "authenticated")
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": id.String(),
		"aud": "authenticated",
		"iss": trimSlash(supabaseURL) + "/auth/v1",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	s, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Parse(s); err != nil {
		t.Fatal(err)
	}
}

func TestRecentLoginUsesSignInTime(t *testing.T) {
	secret := "test-jwt-secret-for-ci-only"
	v, err := NewVerifier(secret, "", "authenticated")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	sign := func(claims jwt.MapClaims) string {
		t.Helper()
		claims["sub"] = uuid.NewString()
		claims["aud"] = "authenticated"
		claims["exp"] = now.Add(time.Hour).Unix()
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		s, err := tok.SignedString([]byte(secret))
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	if err := v.RecentLogin(sign(jwt.MapClaims{"auth_time": now.Unix()}), now, 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := v.RecentLogin(sign(jwt.MapClaims{
		"amr": []map[string]any{{"method": "oauth", "timestamp": now.Unix()}},
	}), now, 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := v.RecentLogin(sign(jwt.MapClaims{
		"auth_time": now.Add(-time.Hour).Unix(),
		"amr":       []map[string]any{{"method": "password", "timestamp": now.Unix()}},
	}), now, 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := v.RecentLogin(sign(jwt.MapClaims{"auth_time": now.Add(-6 * time.Minute).Unix()}), now, 5*time.Minute); err == nil {
		t.Fatal("stale login")
	}
	if err := v.RecentLogin(sign(jwt.MapClaims{}), now, 5*time.Minute); err == nil {
		t.Fatal("missing login time")
	}
}
