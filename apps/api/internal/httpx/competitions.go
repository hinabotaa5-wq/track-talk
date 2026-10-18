package httpx

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
)

func tokyoToday() time.Time {
	now := time.Now().In(jst.Zone)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

func (s *Server) listMajorHosts(w http.ResponseWriter, r *http.Request) {
	rows, err := s.q.ListUpcomingMajorHosts(r.Context(), tokyoToday())
	if err != nil {
		MapDBError(w, err)
		return
	}
	if rows == nil {
		rows = []db.ListUpcomingMajorHostsRow{}
	}
	JSON(w, http.StatusOK, map[string]any{"hosts": rows})
}

func (s *Server) listDomesticMajors(w http.ResponseWriter, r *http.Request) {
	start, end := seasonRange(currentSeasonYear(time.Now()))
	rows, err := s.q.ListDomesticMajors(r.Context(), db.ListDomesticMajorsParams{
		SeasonStart: start,
		SeasonEnd:   end,
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if rows == nil {
		rows = []db.ListDomesticMajorsRow{}
	}
	JSON(w, http.StatusOK, map[string]any{"majors": rows})
}

func (s *Server) listCompetitions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	fields := map[string]string{}
	prefecture, ok := parsePrefectureCode(q.Get("prefecture"))
	if !ok {
		fields["prefecture"] = "地域を選んでください"
	}
	// Without month the whole season comes back, for the month-grouped list on the main tab.
	month := 0
	if raw := q.Get("month"); raw != "" {
		month, ok = parseMonth(raw)
		if !ok {
			fields["month"] = "月を選んでください"
		}
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}

	season := currentSeasonYear(time.Now())
	start, end := seasonRange(season)
	if month != 0 {
		start, end = seasonMonthRange(season, month)
	}
	rows, err := s.q.ListCompetitions(r.Context(), db.ListCompetitionsParams{
		Prefecture: prefecture,
		MonthStart: start,
		MonthEnd:   end,
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if rows == nil {
		rows = []db.Competition{}
	}
	JSON(w, http.StatusOK, map[string]any{
		"competitions": rows,
		"season_year":  season,
		"month":        month,
	})
}

func parsePrefectureCode(raw string) (string, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 0 || n > 47 {
		return "", false
	}
	return fmt.Sprintf("%02d", n), true
}

func parseMonth(raw string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 1 || n > 12 {
		return 0, false
	}
	return n, true
}

// currentSeasonYear is the athletics season that contains now.
// A season starts in April and runs through March of the next calendar year.
func currentSeasonYear(now time.Time) int {
	t := now.In(jst.Zone)
	if t.Month() >= time.April {
		return t.Year()
	}
	return t.Year() - 1
}

// seasonRange returns the half-open date range [start, end) of a whole athletics season, April through March.
func seasonRange(seasonYear int) (start, end time.Time) {
	start = time.Date(seasonYear, time.April, 1, 0, 0, 0, 0, time.UTC)
	return start, start.AddDate(1, 0, 0)
}

// seasonMonthRange returns the half-open date range [start, end) for a month
// inside an athletics season. January through March belong to the next calendar year.
func seasonMonthRange(seasonYear, month int) (start, end time.Time) {
	year := seasonYear
	if month < 4 {
		year++
	}
	start = time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end = start.AddDate(0, 1, 0)
	return start, end
}
