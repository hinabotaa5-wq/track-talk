package main

import (
	"crypto/rand"
	"crypto/rsa"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPushIdentity(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	wrongKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	const audience = "https://budget-stop.run.app"
	const email = "budget-stop@example.iam.gserviceaccount.com"
	authorize := pushAuthorizer(audience, email, func(*jwt.Token) (any, error) { return &key.PublicKey, nil })
	for _, name := range []string{"valid", "bare-issuer", "missing", "malformed", "expired", "no-expiry", "wrong-audience", "wrong-email", "unverified", "wrong-issuer", "wrong-key", "wrong-algorithm"} {
		t.Run(name, func(t *testing.T) {
			claims := jwt.MapClaims{"iss": "https://accounts.google.com", "aud": audience, "email": email, "email_verified": true, "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix()}
			var signingKey any = key
			switch name {
			case "bare-issuer":
				claims["iss"] = "accounts.google.com"
			case "expired":
				claims["exp"] = time.Now().Add(-time.Hour).Unix()
			case "no-expiry":
				delete(claims, "exp")
			case "wrong-audience":
				claims["aud"] = "https://other.run.app"
			case "wrong-email":
				claims["email"] = "other@example.iam.gserviceaccount.com"
			case "unverified":
				claims["email_verified"] = false
			case "wrong-issuer":
				claims["iss"] = "https://attacker.example"
			case "wrong-key":
				signingKey = wrongKey
			}
			var token string
			if name == "wrong-algorithm" {
				token, err = jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("attacker"))
			} else {
				token, err = jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(signingKey)
			}
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			if name == "missing" {
				req.Header.Del("Authorization")
			}
			if name == "malformed" {
				req.Header.Set("Authorization", "Bearer invalid")
			}
			err := authorize(req)
			wantOK := name == "valid" || name == "bare-issuer"
			if (err == nil) != wantOK {
				t.Fatalf("accepted=%v want=%v: %v", err == nil, wantOK, err)
			}
		})
	}
}
