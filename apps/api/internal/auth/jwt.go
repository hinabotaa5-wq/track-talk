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
	issuer string
	jwks   keyfunc.Keyfunc
}

type Identity struct {
	UserID         uuid.UUID
	Nickname       string
	NicknamePublic bool
	// Email はログインに使うメールアドレス（Supabase の JWT の email）。ないこともある。
	Email string
}

func NewVerifier(secret, supabaseURL, aud string) (*Verifier, error) {
	v := &Verifier{secret: []byte(secret), aud: aud}
	if supabaseURL != "" {
		base := trimSlash(supabaseURL)
		v.issuer = base + "/auth/v1"
		jwksURL := v.issuer + "/.well-known/jwks.json"
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

func (v *Verifier) ParseIdentity(tokenString string) (Identity, error) {
	claims, err := v.parseClaims(tokenString)
	if err != nil {
		return Identity{}, err
	}
	sub, _ := claims["sub"].(string)
	id, err := uuid.Parse(sub)
	if err != nil {
		return Identity{}, fmt.Errorf("invalid sub")
	}
	identity := Identity{UserID: id}
	identity.Email, _ = claims["email"].(string)
	if metadata, ok := claims["user_metadata"].(map[string]any); ok {
		identity.Nickname, _ = metadata["nickname"].(string)
		identity.NicknamePublic, _ = metadata["nickname_public"].(bool)
	}
	return identity, nil
}

func (v *Verifier) parseClaims(tokenString string) (jwt.MapClaims, error) {
	opts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{"HS256", "RS256", "ES256", "EdDSA"}),
		jwt.WithExpirationRequired(),
	}
	if v.issuer != "" {
		opts = append(opts, jwt.WithIssuer(v.issuer))
	}
	parser := jwt.NewParser(opts...)
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
		return nil, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	if v.aud != "" && !audienceOK(claims, v.aud) {
		return nil, fmt.Errorf("invalid audience")
	}
	return claims, nil
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
