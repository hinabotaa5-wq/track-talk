package main

import (
	"context"
	"flag"
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
	dryRun := flag.Bool("dry-run", false, "print what would be stored without touching the database")
	flag.Parse()

	_ = godotenv.Load()
	_ = godotenv.Load("../../.env")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	majors, err := competitions.FetchDomesticMajors(ctx, time.Now())
	if err != nil {
		log.Fatal(err)
	}
	if *dryRun {
		for _, major := range majors {
			fmt.Printf("%-20s %s  %s  %s  %s\n", major.Slug, major.ScheduleLabel, major.Name, major.Venue, major.SiteURL)
		}
		return
	}
	if len(majors) == 0 {
		fmt.Println("取り込める大会が無いので前回のデータを残します")
		return
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := storeDomesticMajors(ctx, pool, majors); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("imported %d domestic majors\n", len(majors))
}

// storeDomesticMajors upserts by (slug, year), so editions the run did not see keep their previous rows.
func storeDomesticMajors(ctx context.Context, pool *pgxpool.Pool, majors []competitions.DomesticMajor) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	for _, major := range majors {
		if err := q.UpsertDomesticMajor(ctx, db.UpsertDomesticMajorParams{
			Slug:          major.Slug,
			Year:          int32(major.Year),
			Name:          major.Name,
			Category:      major.Category,
			Venue:         major.Venue,
			StartsOn:      major.StartsOn,
			EndsOn:        major.EndsOn,
			ScheduleLabel: major.ScheduleLabel,
			SiteUrl:       major.SiteURL,
		}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
