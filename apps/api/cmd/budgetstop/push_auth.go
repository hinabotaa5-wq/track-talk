package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// pushAuthorizer accepts only the configured Pub/Sub service account and audience.
// Never derive the expected audience from the caller-controlled Host header.
func pushAuthorizer(audience, email string, key jwt.Keyfunc) func(*http.Request) error {
	return func(r *http.Request) error {
		if audience == "" || email == "" || key == nil {
			return errors.New("push identity is not configured")
		}
		header := strings.Fields(r.Header.Get("Authorization"))
		if len(header) != 2 || !strings.EqualFold(header[0], "Bearer") {
			return errors.New("missing bearer token")
		}
		token, err := jwt.Parse(header[1], key,
			jwt.WithValidMethods([]string{"RS256"}),
			jwt.WithAudience(audience), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
		if err != nil {
			return err
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || !token.Valid {
			return errors.New("invalid token")
		}
		issuer, _ := claims["iss"].(string)
		if issuer != "https://accounts.google.com" && issuer != "accounts.google.com" {
			return errors.New("invalid issuer")
		}
		if claims["email"] != email || claims["email_verified"] != true {
			return errors.New("unauthorized service account")
		}
		return nil
	}
}
