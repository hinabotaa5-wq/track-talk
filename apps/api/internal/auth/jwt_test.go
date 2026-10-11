package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestParseHS256(t *testing.T) {
	secret := "test-jwt-secret-for-ci-only"
	v, err := NewVerifier(secret, "", "authenticated")
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": id.String(),
		"aud": "authenticated",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	s, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	got, err := v.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	if got != id {
		t.Fatalf("got %s", got)
	}
	if _, err := v.Parse("abc"); err == nil {
		t.Fatal("expected error")
	}
}
