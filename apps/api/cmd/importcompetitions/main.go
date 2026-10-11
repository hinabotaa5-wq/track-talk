package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/competitions"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	prefecture := flag.String("prefecture", "", "取り込む都道府県コード。空なら今日の7県まで")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		log.Fatal("GEMINI_API_KEY is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	targets, err := selectTargets(ctx, pool, *prefecture)
	if err != nil {
		log.Fatal(err)
	}
	if len(targets) == 0 {
		fmt.Fprintln(os.Stdout, "今日の取り込み対象はありません")
		return
	}

	client := competitions.Client{Key: key}
	var failed bool
	for i, source := range targets {
		if i > 0 {
			time.Sleep(15 * time.Second)
		}
		if err := importSource(ctx, pool, client, source); err != nil {
			log.Printf("%s: %v", source.Prefecture, err)
			if errors.Is(err, competitions.ErrRateLimited) {
				os.Exit(1)
			}
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

func selectTargets(ctx context.Context, pool *pgxpool.Pool, prefecture string) ([]competitions.Source, error) {
	if prefecture != "" {
		code := prefecture
		if len(code) == 1 {
			code = "0" + code
		}
		for _, source := range competitions.Sources {
			if source.Prefecture == code {
				return []competitions.Source{source}, nil
			}
		}
		return nil, fmt.Errorf("未登録の都道府県です: %s", prefecture)
	}
	updated, err := importTimes(ctx, pool)
	if err != nil {
		return nil, err
	}
	return competitions.DueSources(competitions.Sources, updated, time.Now()), nil
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

func importSource(ctx context.Context, pool *pgxpool.Pool, client competitions.Client, source competitions.Source) error {
	page, err := competitions.FetchPage(ctx, source.PageURL)
	if err != nil {
		return err
	}
	drafts, err := client.Extract(ctx, page)
	if err != nil {
		return err
	}
	meets, err := competitions.BuildMeets(page, drafts, source.Prefecture, source.PageURL)
	if err != nil {
		return err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	if err := q.DeleteCompetitionsBySeason(ctx, db.DeleteCompetitionsBySeasonParams{
		Prefecture: source.Prefecture,
		SeasonYear: int32(meets[0].SeasonYear),
	}); err != nil {
		return err
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
	fmt.Fprintf(os.Stdout, "imported %d competitions for prefecture %s season %d\n", len(meets), source.Prefecture, meets[0].SeasonYear)
	return nil
}
