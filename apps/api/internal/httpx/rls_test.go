package httpx_test

import (
	"context"
	"testing"

	"github.com/hinata/track-talk/apps/api/internal/migrate"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRecruitmentAndPracticeTablesHaveRLS(t *testing.T) {
	dsn := testDSN(t)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	if err := migrate.Up(dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	want := []string{
		"recruitment_details",
		"recruitment_prefectures",
		"recruitment_links",
		"recruitment_photos",
		"practice_details",
		"practice_applications",
		"practice_ratings",
		"guardian_consents",
		"push_subscriptions",
		"coach_profiles",
		"coaching_demo_codes",
		"review_requests",
		"review_offers",
		"review_responses",
		"review_ratings",
		"sense_notes",
		"menu_contest_entries",
		"menu_contest_finalists",
		"menu_contest_votes",
		"schema_migrations",
		"cloudinary_uploads",
	}
	rows, err := pool.Query(ctx, `
		SELECT c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public'
		  AND c.relrowsecurity
		  AND c.relname = ANY($1::text[])
	`, want)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	got := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("%s does not have row level security", name)
		}
	}
}

// public スキーマの表は、公開の anon キーで PostgREST から読み書きされないよう、すべて RLS を有効にする。
// 新しい表で付け忘れたら、ここで落ちる。
func TestEveryPublicTableHasRLS(t *testing.T) {
	dsn := testDSN(t)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	if err := migrate.Up(dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	rows, err := pool.Query(ctx, `
		SELECT c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public'
		  AND c.relkind IN ('r', 'p')
		  AND NOT c.relrowsecurity
		ORDER BY c.relname
	`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		t.Errorf("%s does not have row level security", name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
}
