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
	got, err := v.ParseIdentity(sign(valid))
	if err != nil {
		t.Fatal(err)
	}
	if got.UserID != id {
		t.Fatalf("got %s", got.UserID)
	}
	if _, err := v.ParseIdentity("abc"); err == nil {
		t.Fatal("expected error")
	}
	noExp := jwt.MapClaims{
		"sub": id.String(),
		"aud": "authenticated",
		"iss": supabaseURL + "/auth/v1",
	}
	if _, err := v.ParseIdentity(sign(noExp)); err == nil {
		t.Fatal("missing exp")
	}
	wrongIss := jwt.MapClaims{
		"sub": id.String(),
		"aud": "authenticated",
		"iss": "https://evil.example/auth/v1",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	if _, err := v.ParseIdentity(sign(wrongIss)); err == nil {
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
	if _, err := v.ParseIdentity(s); err != nil {
		t.Fatal(err)
	}
}
