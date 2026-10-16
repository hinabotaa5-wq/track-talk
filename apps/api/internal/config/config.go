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
	// CoachingTickSecret は Cloud Scheduler が添削の tick を呼ぶときのヘッダーの値。空なら tick は使えない。
	CoachingTickSecret string
	// PushHTTP はテストで Web Push の送信先を差し替えるためのもの。nil なら既定のクライアント。
	PushHTTP *http.Client
	// Clock はテストで現在時刻を差し替えるためのもの。nil なら time.Now。
	Clock func() time.Time
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
		CoachingTickSecret:     os.Getenv("COACHING_TICK_SECRET"),
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
