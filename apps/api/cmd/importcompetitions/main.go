package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/competitions"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, competitions.KagoshimaPageURL, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("User-Agent", "TrackTalk/1.0")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		log.Fatalf("kagoshima page: %s", res.Status)
	}
	meets, err := competitions.ParseKagoshima(res.Body, competitions.KagoshimaSourceURL)
	if err != nil {
		log.Fatal(err)
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	if err := q.DeleteCompetitionsBySeason(ctx, db.DeleteCompetitionsBySeasonParams{
		Prefecture: "46",
		SeasonYear: int32(meets[0].SeasonYear),
	}); err != nil {
		log.Fatal(err)
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
			log.Fatal(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(os.Stdout, "imported %d kagoshima competitions for season %d\n", len(meets), meets[0].SeasonYear)
}
