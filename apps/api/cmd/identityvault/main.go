package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/auth"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/identityvault"
)

func fail(stage string) {
	// Connection errors may include addresses or account identifiers.
	slog.Error("identity service startup failed", "stage", stage)
	os.Exit(1)
}

func httpsOrigin(value string) string {
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return ""
	}
	return "https://" + strings.ToLower(u.Host)
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	mainURL := os.Getenv("IDENTITY_MAIN_READ_DATABASE_URL")
	vaultURL := os.Getenv("IDENTITY_DATABASE_URL")
	issuer := httpsOrigin(os.Getenv("IDENTITY_AUTH_URL"))
	mainIssuer := httpsOrigin(os.Getenv("PRIMARY_AUTH_URL"))
	origin := httpsOrigin(os.Getenv("IDENTITY_WEB_ORIGIN"))
	if issuer == "" || mainIssuer == "" || issuer == mainIssuer || origin == "" {
		fail("auth and browser origins")
	}
	if mainURL == "" || vaultURL == "" || mainURL == vaultURL ||
		config.RequireLocalDatabase(mainURL) != nil || config.RequireLocalDatabase(vaultURL) != nil {
		fail("database separation")
	}
	mainDB, err := db.NewPool(ctx, mainURL, "track-talk-identity-read")
	if err != nil {
		fail("main database")
	}
	defer mainDB.Close()
	vaultDB, err := db.NewPool(ctx, vaultURL, "track-talk-identity")
	if err != nil {
		fail("identity database")
	}
	defer vaultDB.Close()
	if mainDB.Ping(ctx) != nil || vaultDB.Ping(ctx) != nil {
		fail("database connectivity")
	}
	var isolated bool
	if err := mainDB.QueryRow(ctx, "SELECT to_regnamespace('identity_vault') IS NULL").Scan(&isolated); err != nil || !isolated {
		fail("identity database isolation")
	}
	verifier, err := auth.NewVerifier("", issuer, "authenticated")
	if err != nil {
		fail("signing keys")
	}
	authClient := auth.Admin{BaseURL: issuer, APIKey: os.Getenv("IDENTITY_AUTH_PUBLIC_KEY")}
	svc, err := identityvault.New(mainDB, vaultDB, verifier, authClient, os.Getenv("COACH_IDENTITY_KEY"),
		os.Getenv("IDENTITY_PUBLIC_READER_TOKEN"), os.Getenv("IDENTITY_CLEANUP_TOKEN"), os.Getenv("IDENTITY_END_TOKEN"), origin)
	if err != nil {
		fail("service configuration")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server := &http.Server{
		Addr: ":" + port, Handler: svc.Handler(),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, IdleTimeout: time.Minute,
	}
	stop, done := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer done()
	go func() {
		<-stop.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fail("listen")
	}
}
