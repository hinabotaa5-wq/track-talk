package main

// 県別（01〜47）の取り込みは廃止した（#549）。このジョブは国際大会（00）の
// 取り込みだけを行い、実行の冒頭で過去シーズンの大会を掃除する。
// 国内主要大会・major-hosts は別のジョブのまま。

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/competitions"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

// OverallTimeout はジョブ全体の期限（ワークフロー側の30分より短く）。
const OverallTimeout = 20 * time.Minute

func main() {
	_ = godotenv.Load()
	_ = godotenv.Load("../../.env")
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), OverallTimeout)
	defer cancel()
	pool, err := db.NewPool(ctx, databaseURL, db.AppNameImport)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	removed, err := db.New(pool).DeletePastCompetitionSeasons(ctx, int32(competitions.SeasonYear(time.Now())))
	if err != nil {
		log.Fatal(err)
	}
	if removed > 0 {
		fmt.Fprintf(os.Stdout, "dropped %d competitions from previous seasons\n", removed)
	}

	updated, err := importTimes(ctx, pool)
	if err != nil {
		log.Fatal(err)
	}
	if !internationalDue(updated, time.Now()) {
		fmt.Fprintln(os.Stdout, "今日の取り込み対象はありません")
		return
	}
	if err := importInternational(ctx, pool); err != nil {
		log.Fatal(err)
	}
}

func importTimes(ctx context.Context, pool *pgxpool.Pool) (map[string]time.Time, error) {
	rows, err := pool.Query(ctx, `SELECT prefecture, MAX(updated_at) FROM competitions GROUP BY prefecture`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	updated := map[string]time.Time{}
	for rows.Next() {
		var prefecture string
		var at time.Time
		if err := rows.Scan(&prefecture, &at); err != nil {
			return nil, err
		}
		updated[prefecture] = at
	}
	return updated, rows.Err()
}

func internationalDue(updated map[string]time.Time, now time.Time) bool {
	imported, ok := updated[competitions.InternationalPrefecture]
	return !ok || imported.IsZero() || now.Sub(imported) >= competitions.InternationalRefreshAfter
}

func importInternational(ctx context.Context, pool *pgxpool.Pool) error {
	now := time.Now()
	meets, err := competitions.FetchInternational(ctx, now)
	if err != nil {
		return err
	}
	_, _, seasons := competitions.InternationalWindow(now)
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	if len(seasons) > 0 {
		if err := q.DeleteCompetitionsBeforeSeason(ctx, db.DeleteCompetitionsBeforeSeasonParams{
			Prefecture: competitions.InternationalPrefecture,
			SeasonYear: int32(seasons[0]),
		}); err != nil {
			return err
		}
	}
	for _, season := range seasons {
		if err := q.DeleteCompetitionsBySeason(ctx, db.DeleteCompetitionsBySeasonParams{
			Prefecture: competitions.InternationalPrefecture,
			SeasonYear: int32(season),
		}); err != nil {
			return err
		}
	}
	for _, meet := range meets {
		if err := q.UpsertCompetition(ctx, db.UpsertCompetitionParams{
			ID:           uuid.New(),
			Name:         meet.Name,
			StartsOn:     meet.StartsOn,
			EndsOn:       meet.EndsOn,
			Location:     meet.Location,
			Prefecture:   meet.Prefecture,
			SeasonYear:   int32(meet.SeasonYear),
			OrganizerUrl: meet.OrganizerURL,
			SourceUrl:    meet.SourceURL,
			LinkLabel:    meet.LinkLabel,
		}); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "imported %d international competitions\n", len(meets))
	return nil
}
