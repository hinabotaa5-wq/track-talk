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

// newsTargets は、ニュースを読む県。確かめたのが古い県から順に並べ、一度も確かめていない県をいちばん先にする。
// DB に大会が1つもない県は要項を付ける先がないので外し、リクエストを使わない。
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

// importNews は、年間の日程表にすでにある大会に、県のニュースから要項を補う。
// 県ごとに Gemini のリクエストを1回使い、使うのは多くても budget 回まで（budget が負なら上限なし）。
// どれかの県が失敗したら true を返す。
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

// newsCandidates は、そのシーズンのすべての大会。ニュースの要項は年間の日程表より新しいので、日程表のものを置き換える。
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
			// 開けない記事が1つあっても、ほかの記事は隠さない。
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
