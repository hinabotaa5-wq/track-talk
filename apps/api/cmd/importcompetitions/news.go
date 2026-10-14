package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/competitions"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// newsTargets are the prefectures to read news for: the ones checked longest ago first, never-checked before any.
// A prefecture with no meets in the database has nothing to attach guidelines to, so it is left out and costs nothing.
func newsTargets(ctx context.Context, pool *pgxpool.Pool, prefecture string, budget int) ([]competitions.Source, error) {
	if prefecture != "" {
		code := prefecture
		if len(code) == 1 {
			code = "0" + code
		}
		for _, source := range competitions.Sources {
			if source.Prefecture == code {
				if source.NewsURL == "" {
					return nil, fmt.Errorf("新着URLが未登録の都道府県です: %s", code)
				}
				return []competitions.Source{source}, nil
			}
		}
		return nil, fmt.Errorf("未登録の都道府県です: %s", prefecture)
	}
	if budget < 1 {
		return nil, nil
	}
	rows, err := db.New(pool).ListNewsCheckTimes(ctx)
	if err != nil {
		return nil, err
	}
	checked := map[string]time.Time{}
	for _, row := range rows {
		checked[row.Prefecture] = row.CheckedAt
	}
	var sources []competitions.Source
	for _, source := range competitions.Sources {
		if source.NewsURL != "" {
			sources = append(sources, source)
		}
	}
	sort.SliceStable(sources, func(i, j int) bool {
		left, leftOK := checked[sources[i].Prefecture]
		right, rightOK := checked[sources[j].Prefecture]
		if leftOK != rightOK {
			return !leftOK
		}
		return left.Before(right)
	})
	return sources, nil
}

// importNews fills in guidelines from prefecture news for meets that the annual schedule already holds.
// It uses one Gemini request per prefecture, at most budget of them (no limit when budget is negative).
// It returns true if any prefecture failed.
func importNews(ctx context.Context, pool *pgxpool.Pool, client competitions.Client, sources []competitions.Source, budget int, dryRun bool) bool {
	q := db.New(pool)
	season := int32(competitions.SeasonYear(time.Now()))
	failed := false
	requests := 0
	for _, source := range sources {
		if budget >= 0 && requests >= budget {
			break
		}
		rows, err := q.ListCompetitionsForNews(ctx, db.ListCompetitionsForNewsParams{Prefecture: source.Prefecture, SeasonYear: season})
		if err != nil {
			log.Printf("%s: %v", source.Prefecture, err)
			return true
		}
		known, byKey := newsCandidates(rows)
		if len(known) == 0 {
			continue
		}
		if requests > 0 {
			time.Sleep(15 * time.Second)
		}
		requests++
		linked, err := readNews(ctx, client, source, int(season), known, byKey, q, dryRun)
		if err != nil {
			log.Printf("%s: %v", source.Prefecture, err)
			if errors.Is(err, competitions.ErrRateLimited) {
				return true
			}
			failed = true
			continue
		}
		if !dryRun {
			if err := q.MarkNewsChecked(ctx, source.Prefecture); err != nil {
				log.Printf("%s: %v", source.Prefecture, err)
				failed = true
			}
		}
		fmt.Fprintf(os.Stdout, "news %s: %d meets got guidelines\n", source.Prefecture, linked)
	}
	return failed
}

// newsCandidates are all meets of the season. Guidelines from news are newer than the annual table's, so they replace it.
func newsCandidates(rows []db.ListCompetitionsForNewsRow) ([]competitions.KnownMeet, map[string]db.ListCompetitionsForNewsRow) {
	var known []competitions.KnownMeet
	byKey := map[string]db.ListCompetitionsForNewsRow{}
	for _, row := range rows {
		key := row.ID.String()
		byKey[key] = row
		known = append(known, competitions.KnownMeet{Key: key, Name: row.Name, StartsOn: row.StartsOn, EndsOn: row.EndsOn, Location: row.Location})
	}
	return known, byKey
}

func readNews(ctx context.Context, client competitions.Client, source competitions.Source, season int, known []competitions.KnownMeet, byKey map[string]db.ListCompetitionsForNewsRow, q *db.Queries, dryRun bool) (int, error) {
	page, err := competitions.FetchNewsList(ctx, source.NewsURL, season)
	if err != nil {
		return 0, err
	}
	items, err := client.ExtractNews(ctx, page)
	if err != nil {
		return 0, err
	}
	linked := 0
	for _, match := range competitions.MatchNews(items, known) {
		link, ok, err := competitions.FindArticleGuidelines(ctx, match.Item)
		if err != nil {
			// One article that cannot be opened must not hide the others.
			log.Printf("%s: %s: %v", source.Prefecture, match.Item.ArticleURL, err)
			continue
		}
		if !ok {
			continue
		}
		row := byKey[match.Meet.Key]
		if dryRun {
			fmt.Fprintf(os.Stdout, "  %s  %s <- %s\n", row.StartsOn.Format("2006-01-02"), row.Name, link)
		} else if err := q.SetCompetitionGuidelines(ctx, db.SetCompetitionGuidelinesParams{ID: row.ID, GuidelinesUrl: link}); err != nil {
			return linked, err
		}
		linked++
	}
	return linked, nil
}
