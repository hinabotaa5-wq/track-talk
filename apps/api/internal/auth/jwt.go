package auth

import (
	"fmt"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Verifier struct {
	secret []byte
	aud    string
	jwks   keyfunc.Keyfunc
}

func NewVerifier(secret, supabaseURL, aud string) (*Verifier, error) {
	v := &Verifier{secret: []byte(secret), aud: aud}
	if supabaseURL != "" {
		jwksURL := trimSlash(supabaseURL) + "/auth/v1/.well-known/jwks.json"
		kf, err := keyfunc.NewDefault([]string{jwksURL})
		if err != nil {
			if len(secret) == 0 {
				return nil, fmt.Errorf("jwks: %w", err)
			}
		} else {
			v.jwks = kf
		}
	}
	return v, nil
}

func (v *Verifier) Parse(tokenString string) (uuid.UUID, error) {
	parser := jwt.NewParser(jwt.WithValidMethods([]string{"HS256", "RS256", "ES256", "EdDSA"}))
	token, err := parser.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() == jwt.SigningMethodHS256.Alg() {
			if len(v.secret) == 0 {
				return nil, fmt.Errorf("hs256 secret not configured")
			}
			return v.secret, nil
		}
		if v.jwks == nil {
			return nil, fmt.Errorf("jwks not configured")
		}
		return v.jwks.Keyfunc(t)
	})
	if err != nil {
		return uuid.Nil, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return uuid.Nil, fmt.Errorf("invalid token")
	}
	if v.aud != "" && !audienceOK(claims, v.aud) {
		return uuid.Nil, fmt.Errorf("invalid audience")
	}
	sub, _ := claims["sub"].(string)
	id, err := uuid.Parse(sub)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid sub")
	}
	return id, nil
}

func audienceOK(claims jwt.MapClaims, want string) bool {
	switch aud := claims["aud"].(type) {
	case string:
		return aud == want
	case []any:
		for _, a := range aud {
			if s, ok := a.(string); ok && s == want {
				return true
			}
		}
	}
	return false
}

func trimSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
