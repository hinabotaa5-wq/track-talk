package httpx

import (
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
)

var errPreferredDates = errors.New("invalid preferred dates")

func availabilityToday(now time.Time) time.Time {
	day, _ := time.Parse("2006-01-02", now.In(jst.Zone).Format("2006-01-02"))
	return day
}

func availabilityDate(value string, today time.Time) (time.Time, bool) {
	day, err := time.Parse("2006-01-02", value)
	return day, err == nil && day.Format("2006-01-02") == value && !day.Before(today) && day.Before(today.AddDate(0, 0, 21))
}

func availabilityJSON(today time.Time, rows []db.CoachAvailability) map[string]any {
	days := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		days = append(days, map[string]any{"date": row.Day.Format("2006-01-02"), "status": row.Status, "note": row.Note})
	}
	return map[string]any{"today": today.Format("2006-01-02"), "days": days}
}

func (s *Server) getPublicCoachAvailability(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	visible, err := s.q.HasPublicAvailabilityPlan(r.Context(), db.HasPublicAvailabilityPlanParams{CoachID: id, PledgeVersion: coachPledgeListedVersion})
	if err != nil {
		MapDBError(w, err)
		return
	}
	today := availabilityToday(time.Now())
	if !visible {
		JSON(w, http.StatusOK, availabilityJSON(today, nil))
		return
	}
	rows, err := s.q.ListCoachAvailability(r.Context(), db.ListCoachAvailabilityParams{CoachID: id, Today: today})
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, availabilityJSON(today, rows))
}

func (s *Server) getMyCoachAvailability(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok || !s.requireCurrentCoach(w, r, me.ID, "承認されたコーチだけが空き状況を確認できます") {
		return
	}
	today := availabilityToday(time.Now())
	rows, err := s.q.ListCoachAvailability(r.Context(), db.ListCoachAvailabilityParams{CoachID: me.ID, Today: today})
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, availabilityJSON(today, rows))
}

func (s *Server) saveCoachAvailability(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok || !s.requireCurrentCoach(w, r, me.ID, "承認されたコーチだけが空き状況を保存できます") {
		return
	}
	var req struct {
		Days []struct {
			Date   string `json:"date"`
			Status string `json:"status"`
			Note   string `json:"note"`
		} `json:"days"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	today := availabilityToday(time.Now())
	fields := map[string]string{}
	if len(req.Days) > 21 {
		fields["days"] = "今日から21日分を入力してください"
	}
	seen := map[string]bool{}
	for i := range req.Days {
		d := &req.Days[i]
		d.Note = strings.TrimSpace(d.Note)
		if _, valid := availabilityDate(d.Date, today); !valid || seen[d.Date] {
			fields["days"] = "今日から21日以内の日を重複せず入力してください"
		}
		seen[d.Date] = true
		if !slices.Contains([]string{"", "available", "consult", "unavailable"}, d.Status) {
			fields["days"] = "空き状況を選んでください"
		}
		if utf8.RuneCountInString(d.Note) > 30 {
			fields["days"] = "メモは30文字以内で入力してください"
		}
		if looksLikeContact(d.Note) {
			fields["days"] = "メモに連絡先は書けません"
		}
		if d.Status == "" && d.Note != "" {
			fields["days"] = "メモを付ける日は空き状況を選んでください"
		}
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	err := s.tx(r.Context(), func(q *db.Queries) error {
		// 申し込み時の希望日の検証と同じロックを使い、保存の途中を読ませない。
		if err := q.LockCoachingPlans(r.Context(), me.ID); err != nil {
			return err
		}
		if err := q.DeletePastCoachAvailability(r.Context(), db.DeletePastCoachAvailabilityParams{CoachID: me.ID, Today: today}); err != nil {
			return err
		}
		for _, d := range req.Days {
			day, _ := availabilityDate(d.Date, today)
			var err error
			if d.Status == "" {
				err = q.DeleteCoachAvailabilityDay(r.Context(), db.DeleteCoachAvailabilityDayParams{CoachID: me.ID, Day: day})
			} else {
				err = q.UpsertCoachAvailability(r.Context(), db.UpsertCoachAvailabilityParams{CoachID: me.ID, Day: day, Status: d.Status, Note: d.Note})
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	s.getMyCoachAvailability(w, r)
}

func preferredDatesJSON(dates []time.Time) []string {
	out := make([]string, 0, len(dates))
	for _, day := range dates {
		out = append(out, day.Format("2006-01-02"))
	}
	return out
}
