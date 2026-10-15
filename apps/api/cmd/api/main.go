package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/auth"
	"github.com/hinata/track-talk/apps/api/internal/cloudinary"
	"github.com/hinata/track-talk/apps/api/internal/competitions"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/httpx"
	"github.com/hinata/track-talk/apps/api/internal/migrate"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	if err := config.RequireLocalDatabase(cfg.DatabaseURL); err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	if err := migrate.Up(cfg.DatabaseURL); err != nil {
		slog.Error("migrate", "err", err)
		os.Exit(1)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		slog.Error("db ping", "err", err)
		os.Exit(1)
	}
	removed, err := db.New(pool).DeletePastCompetitionSeasons(ctx, int32(competitions.SeasonYear(time.Now())))
	if err != nil {
		slog.Error("drop previous competition seasons", "err", err)
		os.Exit(1)
	}
	if removed > 0 {
		slog.Info("dropped previous competition seasons", "rows", removed)
	}
	verifier, err := auth.NewVerifier(cfg.JWTSecret, cfg.SupabaseURL, cfg.JWTAUD)
	if err != nil {
		slog.Error("jwt", "err", err)
		os.Exit(1)
	}
	// Cloud Run は 0 台から起動するたびにここを通るので、Cloudinary の確認は待たずに受け付けを始める。
	go func() {
		presetCtx, presetCancel := context.WithTimeout(ctx, 15*time.Second)
		defer presetCancel()
		if err := (cloudinary.Client{
			Cloud:   cfg.CloudName,
			Key:     cfg.CloudAPIKey,
			Secret:  cfg.CloudAPISecret,
			APIBase: cfg.CloudAPIBase,
		}).EnsureUploadPresets(presetCtx); err != nil {
			slog.Error("cloudinary presets", "err", err)
		}
	}()
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpx.New(cfg, pool, verifier),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		slog.Info("listen", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http", "err", err)
			os.Exit(1)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shutCtx, shutCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutCancel()
	_ = srv.Shutdown(shutCtx)
}
