package httpx

import (
	"net/http"
	"sort"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
)

// 管理者画面の「数字」タブ。件数と人数だけを返し、本文やアカウントは返さない。
const (
	adminStatsWeeks = 12
	// 機能ごとの利用と、問い合わせ・通報の内訳で「直近」とする日数。
	adminStatsRecentDays = 30
	// コーチングの需要と供給を見る日数。
	adminCoachingDays = 90
)

// weekStartJST は、t を含む週の月曜 0時(日本時間)。
func weekStartJST(t time.Time) time.Time {
	local := t.In(jst.Zone)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, jst.Zone)
	offset := (int(day.Weekday()) + 6) % 7
	return day.AddDate(0, 0, -offset)
}

func (s *Server) adminStats(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	ctx := r.Context()
	now := s.now()
	since := weekStartJST(now).AddDate(0, 0, -7*(adminStatsWeeks-1))
	recent := now.AddDate(0, 0, -adminStatsRecentDays)

	weeks, err := s.q.AdminWeeklyStats(ctx, db.AdminWeeklyStatsParams{Since: since, Until: now})
	if err != nil {
		MapDBError(w, err)
		return
	}
	// 5週目まで見るので、登録した週は12週より前からさかのぼる。
	retention, err := s.q.AdminRetention(ctx, db.AdminRetentionParams{Since: since.AddDate(0, 0, -35), Now: now})
	if err != nil {
		MapDBError(w, err)
		return
	}
	features, err := s.q.AdminFeatureUsage(ctx, recent)
	if err != nil {
		MapDBError(w, err)
		return
	}
	inquiries, err := s.q.AdminInquiryBreakdown(ctx, recent)
	if err != nil {
		MapDBError(w, err)
		return
	}
	reports, err := s.q.AdminReportBreakdown(ctx, recent)
	if err != nil {
		MapDBError(w, err)
		return
	}

	weekRows := make([]map[string]any, 0, len(weeks))
	for _, row := range weeks {
		weekRows = append(weekRows, map[string]any{
			"week_start":   row.WeekStart.Format(time.DateOnly),
			"signups":      row.Signups,
			"active_users": row.ActiveUsers,
			"posts":        row.Posts,
			"sense_notes":  row.SenseNotes,
			"race_records": row.RaceRecords,
			"deletions":    row.Deletions,
		})
	}
	retentionRows := make([]map[string]any, 0, len(retention))
	for _, row := range retention {
		retentionRows = append(retentionRows, map[string]any{
			"week_start": row.WeekStart.Format(time.DateOnly),
			"signups":    row.Signups,
			"retained":   row.Retained,
		})
	}
	JSON(w, http.StatusOK, map[string]any{
		"recent_days": adminStatsRecentDays,
		"weeks":       weekRows,
		"retention":   retentionRows,
		"features":    features,
		"inquiries":   inquiries,
		"reports":     reports,
	})
}

// adminCoachingStats は、コーチングの種目ごとの需要(添削の依頼・パーソナル指導の申し込み)と供給(承認済みのコーチ)。
// 提供を始める前は、デモの招待コードで試した分だけが入る。
func (s *Server) adminCoachingStats(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	ctx := r.Context()
	since := s.now().AddDate(0, 0, -adminCoachingDays)
	supply, err := s.q.AdminCoachSupply(ctx)
	if err != nil {
		MapDBError(w, err)
		return
	}
	reviews, err := s.q.AdminReviewDemand(ctx, since)
	if err != nil {
		MapDBError(w, err)
		return
	}
	contracts, err := s.q.AdminContractDemand(ctx, since)
	if err != nil {
		MapDBError(w, err)
		return
	}

	type eventRow struct {
		Event          string  `json:"event"`
		EventLabel     string  `json:"event_label"`
		Coaches        int64   `json:"coaches"`
		OnDuty         int64   `json:"on_duty"`
		Requests       int64   `json:"requests"`
		Returned       int64   `json:"returned"`
		Expired        int64   `json:"expired"`
		MedianHours    float64 `json:"median_hours"`
		Rated          int64   `json:"rated"`
		AvgStars       float64 `json:"avg_stars"`
		HelpfulAnswers int64   `json:"helpful_answers"`
		HelpfulYes     int64   `json:"helpful_yes"`
		Applied        int64   `json:"applied"`
		Started        int64   `json:"started"`
		Lost           int64   `json:"lost"`
		StartedYen     int64   `json:"started_yen"`
	}
	byEvent := map[string]*eventRow{}
	get := func(event string) *eventRow {
		if row, ok := byEvent[event]; ok {
			return row
		}
		row := &eventRow{Event: event, EventLabel: eventLabel(event)}
		byEvent[event] = row
		return row
	}
	for _, row := range supply {
		item := get(row.Event)
		item.Coaches, item.OnDuty = row.Coaches, row.OnDuty
	}
	for _, row := range reviews {
		item := get(row.Event)
		item.Requests, item.Returned, item.Expired, item.MedianHours = row.Requests, row.Returned, row.Expired, row.MedianHours
		item.Rated, item.AvgStars, item.HelpfulAnswers, item.HelpfulYes = row.Rated, row.AvgStars, row.HelpfulAnswers, row.HelpfulYes
	}
	for _, row := range contracts {
		item := get(row.Event)
		item.Applied, item.Started, item.Lost, item.StartedYen = row.Applied, row.Started, row.Lost, row.StartedYen
	}
	// 種目の並びはアプリ内の種目の順。一覧にない古いコードは後ろに付ける。
	out := make([]eventRow, 0, len(byEvent))
	for _, event := range appEvents {
		if row, ok := byEvent[event.Code]; ok {
			out = append(out, *row)
			delete(byEvent, event.Code)
		}
	}
	rest := make([]string, 0, len(byEvent))
	for event := range byEvent {
		rest = append(rest, event)
	}
	sort.Strings(rest)
	for _, event := range rest {
		out = append(out, *byEvent[event])
	}
	JSON(w, http.StatusOK, map[string]any{"days": adminCoachingDays, "events": out})
}
