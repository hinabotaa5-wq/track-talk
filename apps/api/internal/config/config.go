package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Addr            string
	DatabaseURL     string
	SupabaseURL     string
	JWTSecret       string
	JWTAUD          string
	CORSOrigins     []string
	LogLevel        string
	ShutdownTimeout time.Duration
	CloudName       string
	CloudAPIKey     string
	CloudAPISecret  string
	CloudAPIBase    string
}

func Load() (Config, error) {
	_ = godotenv.Load()
	_ = godotenv.Load("../../.env")

	cfg := Config{
		Addr:            env("API_ADDR", ":8080"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		SupabaseURL:     os.Getenv("SUPABASE_URL"),
		JWTSecret:       os.Getenv("SUPABASE_JWT_SECRET"),
		JWTAUD:          env("SUPABASE_JWT_AUD", "authenticated"),
		LogLevel:        env("LOG_LEVEL", "info"),
		ShutdownTimeout: 10 * time.Second,
		CloudName:       os.Getenv("CLOUDINARY_CLOUD_NAME"),
		CloudAPIKey:     os.Getenv("CLOUDINARY_API_KEY"),
		CloudAPISecret:  os.Getenv("CLOUDINARY_API_SECRET"),
		CloudAPIBase:    os.Getenv("CLOUDINARY_API_BASE"),
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
