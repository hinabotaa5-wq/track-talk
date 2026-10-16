package config

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Addr                   string
	DatabaseURL            string
	SupabaseURL            string
	SupabaseServiceRoleKey string
	JWTSecret              string
	JWTAUD                 string
	CORSOrigins            []string
	LogLevel               string
	ShutdownTimeout        time.Duration
	CloudName              string
	CloudAPIKey            string
	CloudAPISecret         string
	CloudAPIBase           string
	VAPIDPublicKey         string
	VAPIDPrivateKey        string
	VAPIDSubject           string
	// CoachingEnabled が false の間は、オンラインコーチングの API（年齢・保護者の同意・
	// プッシュ通知・添削動画）を 404 にする。提供を始めるまで本番では立てない。
	CoachingEnabled bool
	// PushHTTP はテストで Web Push の送信先を差し替えるためのもの。nil なら既定のクライアント。
	PushHTTP *http.Client
}

func Load() (Config, error) {
	_ = godotenv.Load()
	_ = godotenv.Load("../../.env")

	cfg := Config{
		Addr:                   env("API_ADDR", ":8080"),
		DatabaseURL:            os.Getenv("DATABASE_URL"),
		SupabaseURL:            os.Getenv("SUPABASE_URL"),
		SupabaseServiceRoleKey: os.Getenv("SUPABASE_SERVICE_ROLE_KEY"),
		JWTSecret:              os.Getenv("SUPABASE_JWT_SECRET"),
		JWTAUD:                 env("SUPABASE_JWT_AUD", "authenticated"),
		LogLevel:               env("LOG_LEVEL", "info"),
		ShutdownTimeout:        10 * time.Second,
		CloudName:              os.Getenv("CLOUDINARY_CLOUD_NAME"),
		CloudAPIKey:            os.Getenv("CLOUDINARY_API_KEY"),
		CloudAPISecret:         os.Getenv("CLOUDINARY_API_SECRET"),
		CloudAPIBase:           os.Getenv("CLOUDINARY_API_BASE"),
		VAPIDPublicKey:         os.Getenv("VAPID_PUBLIC_KEY"),
		VAPIDPrivateKey:        os.Getenv("VAPID_PRIVATE_KEY"),
		VAPIDSubject:           os.Getenv("VAPID_SUBJECT"),
		CoachingEnabled:        enabled(os.Getenv("COACHING_ENABLED")),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" && cfg.SupabaseURL == "" {
		return Config{}, fmt.Errorf("SUPABASE_JWT_SECRET or SUPABASE_URL is required")
	}
	origins := env("CORS_ORIGINS", "http://localhost:3000")
	for _, o := range strings.Split(origins, ",") {
		if v := strings.TrimSpace(o); v != "" {
			cfg.CORSOrigins = append(cfg.CORSOrigins, v)
		}
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func enabled(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
