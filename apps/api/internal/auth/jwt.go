package auth

import (
	"encoding/json"
	"fmt"
	"time"

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

func (v *Verifier) Parse(tokenString string) (uuid.UUID, error) {
	identity, err := v.ParseIdentity(tokenString)
	if err != nil {
		return uuid.Nil, err
	}
	return identity.UserID, nil
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
	if metadata, ok := claims["user_metadata"].(map[string]any); ok {
		identity.Nickname, _ = metadata["nickname"].(string)
		identity.NicknamePublic, _ = metadata["nickname_public"].(bool)
	}
	return identity, nil
}

// RecentLogin reports whether the token was issued by a sign-in within maxAge.
// A refreshed access token keeps the original sign-in time, so it does not count.
func (v *Verifier) RecentLogin(tokenString string, now time.Time, maxAge time.Duration) error {
	claims, err := v.parseClaims(tokenString)
	if err != nil {
		return err
	}
	at, ok := loginTime(claims)
	if !ok || now.Sub(at) > maxAge {
		return fmt.Errorf("login is not recent")
	}
	return nil
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

func loginTime(claims jwt.MapClaims) (time.Time, bool) {
	var latest time.Time
	if at, ok := numericTime(claims["auth_time"]); ok {
		latest = at
	}
	switch amr := claims["amr"].(type) {
	case []any:
		for _, item := range amr {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if at, ok := numericTime(entry["timestamp"]); ok && at.After(latest) {
				latest = at
			}
		}
	}
	return latest, !latest.IsZero()
}

func numericTime(value any) (time.Time, bool) {
	switch n := value.(type) {
	case float64:
		return time.Unix(int64(n), 0), true
	case int64:
		return time.Unix(n, 0), true
	case json.Number:
		unix, err := n.Int64()
		if err != nil {
			return time.Time{}, false
		}
		return time.Unix(unix, 0), true
	default:
		return time.Time{}, false
	}
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
