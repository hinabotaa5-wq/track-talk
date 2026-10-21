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

// FetchDomesticMajors は、日本陸連の今シーズンと次のシーズンの月ごとの一覧を読み、DomesticMeets のどれかに当たる行の詳細ページを読む。
// 読めないページはログに残して飛ばし、1か所が落ちていてもほかの大会は残す。
func FetchDomesticMajors(ctx context.Context, now time.Time) ([]DomesticMajor, error) {
	// 年度は日本時間で決める(9時前に手で流すと、UTC ではまだ前の月のことがある)。
	season := SeasonYear(now)
	meets := map[string]DomesticMeet{}
	for _, meet := range DomesticMeets {
		meets[meet.Slug] = meet
	}

	type target struct {
		meet DomesticMeet
		path string
		year int
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
			targets = append(targets, target{meet: meet, path: entry.Path, year: month.Year()})
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
		major, err := ParseJAAFDetail(bytes.NewReader(body), detailURL, t.year)
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
	majors = append(majors, fetchRegionalIncare(ctx, season)...)
	sort.SliceStable(majors, func(i, j int) bool { return majors[i].StartsOn.Before(majors[j].StartsOn) })
	return majors, nil
}

// seasonMonths は、season 年の4月から count シーズン分のすべての月を並べる。
func seasonMonths(season, count int) []time.Time {
	var months []time.Time
	for i := 0; i < count*12; i++ {
		months = append(months, time.Date(season, time.April+time.Month(i), 1, 0, 0, 0, 0, time.UTC))
	}
	return months
}

// fetchRegionalIncare は、8つの連盟の今年度と次年度のページを読む。
// 次年度のページは公開が遅いので、ないページはエラーにしない。
func fetchRegionalIncare(ctx context.Context, season int) []DomesticMajor {
	var majors []DomesticMajor
	for _, fiscalYear := range []int{season, season + 1} {
		for federation, slug := range regionalIncare {
			pageURL := iuauPageURL(fiscalYear, federation)
			body, err := getBody(ctx, pageURL)
			if err != nil {
				log.Printf("domestic majors: %v", err)
				continue
			}
			page, err := decodeShiftJIS(body)
			if err != nil {
				log.Printf("domestic majors: %s: %v", pageURL, err)
				continue
			}
			major, err := ParseRegionalIncare(page, fiscalYear, pageURL)
			if err != nil {
				log.Printf("domestic majors: %s: %v", pageURL, err)
				continue
			}
			if major == nil {
				continue
			}
			major.Slug = slug
			majors = append(majors, *major)
		}
	}
	return majors
}
