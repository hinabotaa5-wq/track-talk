package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/competitions"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	_ = godotenv.Load("../../.env")
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := db.NewPool(ctx, databaseURL, db.AppNameImport)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	now := time.Now()
	failed := false
	for _, kind := range []string{competitions.MajorWorlds, competitions.MajorOlympics} {
		hosts, err := competitions.FetchMajorHosts(ctx, kind, now)
		if err != nil {
			log.Printf("%s: %v", kind, err)
			failed = true
			continue
		}
		next, replace := competitions.MajorHostsForStore(hosts)
		if !replace {
			fmt.Fprintf(os.Stdout, "%s: 今後の開催地が無いので前回のデータを残します\n", kind)
			continue
		}
		if err := storeMajorHosts(ctx, pool, kind, next); err != nil {
			log.Printf("%s: %v", kind, err)
			failed = true
			continue
		}
		fmt.Fprintf(os.Stdout, "imported %d %s hosts\n", len(next), kind)
	}
	if failed {
		os.Exit(1)
	}
}

func storeMajorHosts(ctx context.Context, pool *pgxpool.Pool, kind string, hosts []competitions.MajorHost) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	years := make([]int32, 0, len(hosts))
	for _, host := range hosts {
		years = append(years, int32(host.Year))
	}
	if err := q.DeleteMajorHostsNotInYears(ctx, db.DeleteMajorHostsNotInYearsParams{Kind: kind, Years: years}); err != nil {
		return err
	}
	for _, host := range hosts {
		if err := q.UpsertMajorHost(ctx, db.UpsertMajorHostParams{
			Kind:          host.Kind,
			Year:          int32(host.Year),
			Name:          host.Name,
			City:          host.City,
			Country:       host.Country,
			Venue:         host.Venue,
			StartsOn:      host.StartsOn,
			EndsOn:        host.EndsOn,
			ScheduleLabel: host.ScheduleLabel,
			ImageUrl:      host.ImageURL,
			PageUrl:       host.PageURL,
		}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
