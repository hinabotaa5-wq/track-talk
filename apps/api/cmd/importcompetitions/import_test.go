package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/competitions"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/db"
	appmigrate "github.com/hinata/track-talk/apps/api/internal/migrate"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Issue #460 (b): 複数 URL の県で一部 URL が失敗しても、成功分が取り込まれ、
// 失敗した URL の既存データ（ニュースで補った要項つき）が消えない。
func TestImportSourcePartialWritesSuccessAndPreservesFailed(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://tracktalk:tracktalk@127.0.0.1:55432/tracktalk?sslmode=disable"
	}
	if err := config.RequireLocalDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	defer admin.Close(ctx)
	name := "import_partial_" + uuid.New().String()[:8]
	ident := pgx.Identifier{name}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+ident); err != nil {
		t.Fatalf("create isolated test database: %v", err)
	}
	defer func() {
		if _, err := admin.Exec(ctx, "DROP DATABASE "+ident+" WITH (FORCE)"); err != nil {
			t.Errorf("drop test database: %v", err)
		}
	}()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, "SELECT 1"); err != nil {
		t.Fatal(err)
	}
	// scratch DB の URL を組み立てる（migrate 用）。
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Path = "/" + name
	if err := appmigrate.Up(parsed.String()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(partialSchedulePage("GOOD記録会")))
	}))
	defer good.Close()
	badPage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer badPage.Close()
	gemini := partialGeminiServer(t, "")
	defer gemini.Close()

	q := db.New(pool)
	oldID := uuid.New()
	if err := q.UpsertCompetition(ctx, db.UpsertCompetitionParams{
		ID:           oldID,
		Name:         "OLD記録会",
		StartsOn:     time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		Prefecture:   "22",
		SeasonYear:   2026,
		OrganizerUrl: badPage.URL,
		SourceUrl:    badPage.URL,
	}); err != nil {
		t.Fatal(err)
	}
	if err := q.SetCompetitionGuidelines(ctx, db.SetCompetitionGuidelinesParams{ID: oldID, GuidelinesUrl: "https://example.com/yoko.pdf"}); err != nil {
		t.Fatal(err)
	}

	source := competitions.Source{Prefecture: "22", PageURL: good.URL, ExtraURLs: []string{badPage.URL}}
	outcome, err := importSource(ctx, pool, competitions.Client{Key: "test", Endpoint: gemini.URL}, source, nil)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.result != competitions.AttemptPartial {
		t.Fatalf("result %q", outcome.result)
	}

	rows, err := q.ListCompetitionsForNews(ctx, db.ListCompetitionsForNewsParams{Prefecture: "22", SeasonYear: 2026})
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]db.ListCompetitionsForNewsRow{}
	for _, row := range rows {
		byName[row.Name] = row
	}
	goodRow, ok := byName["GOOD記録会"]
	if !ok {
		t.Fatalf("good meet was not imported: %+v", rows)
	}
	if goodRow.SourceUrl != good.URL {
		t.Fatalf("good source %q", goodRow.SourceUrl)
	}
	oldRow, ok := byName["OLD記録会"]
	if !ok {
		t.Fatalf("failed url data was deleted: %+v", rows)
	}
	if oldRow.GuidelinesUrl != "https://example.com/yoko.pdf" {
		t.Fatalf("news guidelines lost: %+v", oldRow)
	}
}
