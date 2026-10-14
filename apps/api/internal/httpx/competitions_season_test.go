package httpx_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestDeleteCompetitionsBeforeSeason(t *testing.T) {
	setup(t)
	ctx := context.Background()
	pool := openPool(t)
	q := db.New(pool)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM competitions WHERE prefecture IN ('46', '47')`)
	})

	insertSeason := func(prefecture string, season int) {
		t.Helper()
		start := time.Date(season, time.October, 10, 0, 0, 0, 0, time.UTC)
		if err := q.UpsertCompetition(ctx, db.UpsertCompetitionParams{
			ID:           uuid.New(),
			Name:         "県選手権",
			StartsOn:     start,
			Location:     "競技場",
			Prefecture:   prefecture,
			SeasonYear:   int32(season),
			OrganizerUrl: "https://example.com/meet",
			SourceUrl:    "https://example.com/list",
			LinkLabel:    "要項",
		}); err != nil {
			t.Fatal(err)
		}
	}
	insertSeason("47", 2025)
	insertSeason("47", 2026)
	insertSeason("47", 2027)
	insertSeason("46", 2025)

	if err := q.DeleteCompetitionsBeforeSeason(ctx, db.DeleteCompetitionsBeforeSeasonParams{
		Prefecture: "47",
		SeasonYear: 2026,
	}); err != nil {
		t.Fatal(err)
	}
	got := seasonYears(t, pool, "47")
	if len(got) != 2 || got[0] != 2026 || got[1] != 2027 {
		t.Fatalf("prefecture 47 seasons %v", got)
	}
	other := seasonYears(t, pool, "46")
	if len(other) != 1 || other[0] != 2025 {
		t.Fatalf("prefecture 46 seasons %v", other)
	}
}

func TestDeletePastCompetitionSeasonsKeepsCurrentAndNext(t *testing.T) {
	setup(t)
	ctx := context.Background()
	pool := openPool(t)
	q := db.New(pool)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM competitions WHERE prefecture IN ('44', '45')`)
	})
	now := time.Now().In(time.FixedZone("Asia/Tokyo", 9*60*60))
	season := now.Year()
	if now.Month() < time.April {
		season--
	}
	for _, item := range []struct {
		prefecture string
		year       int
	}{
		{"44", season - 1},
		{"44", season},
		{"45", season - 1},
		{"45", season + 1},
	} {
		if err := q.UpsertCompetition(ctx, db.UpsertCompetitionParams{
			ID:           uuid.New(),
			Name:         "シーズン確認",
			StartsOn:     time.Date(item.year, time.October, 10, 0, 0, 0, 0, time.UTC),
			Location:     "競技場",
			Prefecture:   item.prefecture,
			SeasonYear:   int32(item.year),
			OrganizerUrl: "https://example.com/meet",
			SourceUrl:    "https://example.com/list",
			LinkLabel:    "要項",
		}); err != nil {
			t.Fatal(err)
		}
	}
	removed, err := q.DeletePastCompetitionSeasons(ctx, int32(season))
	if err != nil {
		t.Fatal(err)
	}
	if removed < 2 {
		t.Fatalf("removed %d", removed)
	}
	if got := seasonYears(t, pool, "44"); len(got) != 1 || got[0] != int32(season) {
		t.Fatalf("prefecture 44 %v", got)
	}
	if got := seasonYears(t, pool, "45"); len(got) != 1 || got[0] != int32(season+1) {
		t.Fatalf("prefecture 45 %v", got)
	}
}

func TestMigrationDropsPreviousCompetitionSeasons(t *testing.T) {
	setup(t)
	ctx := context.Background()
	pool := openPool(t)
	q := db.New(pool)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM competitions WHERE name = '過去シーズン確認'`)
	})

	now := time.Now().In(time.FixedZone("Asia/Tokyo", 9*60*60))
	season := now.Year()
	if now.Month() < time.April {
		season--
	}
	for _, year := range []int{season - 1, season, season + 1} {
		if err := q.UpsertCompetition(ctx, db.UpsertCompetitionParams{
			ID:           uuid.New(),
			Name:         "過去シーズン確認",
			StartsOn:     time.Date(year, time.October, 10, 0, 0, 0, 0, time.UTC),
			Location:     "競技場",
			Prefecture:   "45",
			SeasonYear:   int32(year),
			OrganizerUrl: "https://example.com/meet",
			SourceUrl:    "https://example.com/list",
			LinkLabel:    "要項",
		}); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := pool.Exec(ctx, `
DELETE FROM competitions
WHERE season_year < (
    CASE
        WHEN EXTRACT(MONTH FROM (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Tokyo')) >= 4
            THEN EXTRACT(YEAR FROM (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Tokyo'))
        ELSE EXTRACT(YEAR FROM (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Tokyo')) - 1
    END
)::int`); err != nil {
		t.Fatal(err)
	}
	got := seasonYears(t, pool, "45")
	if len(got) != 2 || got[0] != int32(season) || got[1] != int32(season+1) {
		t.Fatalf("seasons %v, current %d", got, season)
	}
}

func openPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://tracktalk:tracktalk@localhost:5432/tracktalk?sslmode=disable"
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func seasonYears(t *testing.T, pool *pgxpool.Pool, prefecture string) []int32 {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT season_year FROM competitions WHERE prefecture = $1 ORDER BY season_year`, prefecture)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var years []int32
	for rows.Next() {
		var year int32
		if err := rows.Scan(&year); err != nil {
			t.Fatal(err)
		}
		years = append(years, year)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return years
}
