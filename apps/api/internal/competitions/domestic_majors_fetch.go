package competitions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"time"
)

// FetchDomesticMajors reads the JAAF monthly lists of this season and the next,
// then the detail page of every entry that is one of the DomesticMeets.
// A page that cannot be read is logged and skipped so one outage keeps the other meets.
func FetchDomesticMajors(ctx context.Context, now time.Time) ([]DomesticMajor, error) {
	season := now.Year()
	if now.Month() < time.April {
		season--
	}
	meets := map[string]DomesticMeet{}
	for _, meet := range DomesticMeets {
		meets[meet.Slug] = meet
	}

	type target struct {
		meet DomesticMeet
		path string
	}
	var targets []target
	seen := map[string]bool{}
	listed := 0
	var failures []error
	for _, month := range seasonMonths(season, 2) {
		listURL := fmt.Sprintf("%s/competition/list/?year=%d&month=%d", jaafOrigin, month.Year(), int(month.Month()))
		body, err := getBody(ctx, listURL)
		if err != nil {
			failures = append(failures, err)
			log.Printf("domestic majors: %v", err)
			continue
		}
		entries, err := ParseJAAFList(bytes.NewReader(body))
		if err != nil {
			failures = append(failures, err)
			continue
		}
		listed++
		for _, entry := range entries {
			meet, ok := MatchDomesticMeet(entry.Title)
			if !ok || seen[entry.Path] {
				continue
			}
			seen[entry.Path] = true
			targets = append(targets, target{meet: meet, path: entry.Path})
		}
	}
	if listed == 0 {
		return nil, fmt.Errorf("JAAFの大会一覧を読めません: %w", errors.Join(failures...))
	}

	var majors []DomesticMajor
	for _, t := range targets {
		detailURL := jaafOrigin + t.path
		body, err := getBody(ctx, detailURL)
		if err != nil {
			log.Printf("domestic majors: %v", err)
			continue
		}
		major, err := ParseJAAFDetail(bytes.NewReader(body), detailURL)
		if errors.Is(err, errScheduleUndecided) {
			continue
		}
		if err != nil {
			log.Printf("domestic majors: %v", err)
			continue
		}
		major.Slug = t.meet.Slug
		major.Category = t.meet.Category
		majors = append(majors, major)
	}
	sort.SliceStable(majors, func(i, j int) bool { return majors[i].StartsOn.Before(majors[j].StartsOn) })
	return majors, nil
}

// seasonMonths lists every month of count seasons starting in April of season.
func seasonMonths(season, count int) []time.Time {
	var months []time.Time
	for i := 0; i < count*12; i++ {
		months = append(months, time.Date(season, time.April+time.Month(i), 1, 0, 0, 0, 0, time.UTC))
	}
	return months
}
