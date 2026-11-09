package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

// 管理画面の感覚メモ集計。件数・人数・日付だけで、本文やアカウントは返さない。
// 合成データで受入条件を確かめる：分布バケット、同日の複数件は1日、
// 7日リピートの分母は期間経過者のみ、28日分母ゼロ、コホートの翌週窓と月曜始まり。
func TestAdminSenseNoteStats(t *testing.T) {
	fixed := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	h := setupConfig(t, func(cfg *config.Config) {
		cfg.Clock = func() time.Time { return fixed }
	})
	adminID := uuid.New()
	admin := token(t, adminID)
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", admin, nil); rec.Code != http.StatusOK {
		t.Fatalf("admin profile %d", rec.Code)
	}
	setAdmin(t, adminID)

	// day -> その日に書く著者ごとの件数。直接 SQL で入れる（画面は1日1件だが、集計は同日の複数件も数える）。
	addNotes := func(id uuid.UUID, days map[string]int) {
		t.Helper()
		if rec := doJSON(t, h, http.MethodGet, "/v1/me", token(t, id), nil); rec.Code != http.StatusOK {
			t.Fatalf("profile %d", rec.Code)
		}
		for day, n := range days {
			for range n {
				execSQLArgs(t, `INSERT INTO sense_notes (id, author_id, body, day) VALUES (gen_random_uuid(), $1, '秘密の感覚メモ本文', $2::date)`, id, day)
			}
		}
	}
	newID := func() uuid.UUID { return uuid.New() }
	adminNote := map[string]int{"2026-06-20": 1}
	a, b, c, d := newID(), newID(), newID(), newID()
	e, f, g, hh := newID(), newID(), newID(), newID()
	i, j, k, l := newID(), newID(), newID(), newID()
	m, n, o, p := newID(), newID(), newID(), newID()
	addNotes(adminID, adminNote)
	addNotes(a, map[string]int{"2026-06-01": 1})
	addNotes(b, map[string]int{"2026-06-02": 1, "2026-06-03": 1, "2026-06-10": 1})
	addNotes(c, map[string]int{"2026-05-20": 1, "2026-05-21": 1, "2026-05-22": 1, "2026-05-23": 1, "2026-05-24": 1, "2026-05-25": 1})
	addNotes(d, map[string]int{
		"2026-05-01": 1, "2026-05-02": 1, "2026-05-03": 1, "2026-05-04": 1, "2026-05-05": 1, "2026-05-06": 1,
		"2026-05-07": 1, "2026-05-08": 1, "2026-05-09": 1, "2026-05-10": 1, "2026-05-11": 1,
	})
	addNotes(e, map[string]int{"2026-06-05": 3})
	addNotes(f, map[string]int{"2026-06-06": 2, "2026-06-07": 1})
	addNotes(g, map[string]int{
		"2026-06-08": 1, "2026-06-09": 1, "2026-06-10": 1, "2026-06-11": 1,
		"2026-06-12": 1, "2026-06-13": 1, "2026-06-14": 1,
	})
	addNotes(hh, map[string]int{"2026-06-23": 1, "2026-06-24": 1})
	addNotes(i, map[string]int{"2026-06-10": 2})
	addNotes(j, map[string]int{"2026-06-17": 1, "2026-06-24": 1})
	addNotes(k, map[string]int{"2026-06-01": 1, "2026-06-09": 1})
	addNotes(l, map[string]int{"2026-05-27": 1, "2026-06-24": 1})
	addNotes(m, map[string]int{"2026-06-21": 1, "2026-06-22": 1})
	addNotes(n, map[string]int{"2026-06-22": 1})
	addNotes(o, map[string]int{"2026-06-15": 1, "2026-06-29": 1})
	addNotes(p, map[string]int{"2026-06-11": 1, "2026-06-12": 1, "2026-06-13": 1})

	member := token(t, newID())
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", member, nil); rec.Code != http.StatusOK {
		t.Fatalf("member profile %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/admin/stats", member, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("member stats = %d", rec.Code)
	}

	rec := doJSON(t, h, http.MethodGet, "/v1/admin/stats", admin, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("stats %d %s", rec.Code, rec.Body.String())
	}
	var stats struct {
		Weeks []struct {
			WeekStart string `json:"week_start"`
		} `json:"weeks"`
		SenseNotes struct {
			Total            int     `json:"total"`
			Authors          int     `json:"authors"`
			NonAdminAuthors  int     `json:"non_admin_authors"`
			Profiles         int     `json:"profiles"`
			NonAdminProfiles int     `json:"non_admin_profiles"`
			FirstDay         *string `json:"first_day"`
			LastDay          *string `json:"last_day"`
			Buckets          struct {
				One      int `json:"one"`
				TwoFour  int `json:"two_four"`
				FiveNine int `json:"five_nine"`
				TenPlus  int `json:"ten_plus"`
			} `json:"buckets"`
			ActiveDays struct {
				Days2Plus int `json:"days_2_plus"`
				Days3Plus int `json:"days_3_plus"`
				Days7Plus int `json:"days_7_plus"`
			} `json:"active_days"`
			Repeat7  struct{ Total, Repeat int } `json:"repeat_7"`
			Repeat28 struct{ Total, Repeat int } `json:"repeat_28"`
			Cohorts  []struct {
				WeekStart     string `json:"week_start"`
				Authors       int    `json:"authors"`
				NextWeek      int    `json:"next_week"`
				SinceNextWeek int    `json:"since_next_week"`
			} `json:"cohorts"`
			Weekly []struct {
				WeekStart string `json:"week_start"`
				Notes     int    `json:"notes"`
				Authors   int    `json:"authors"`
			} `json:"weekly"`
			Monthly []struct {
				Month   string `json:"month"`
				Notes   int    `json:"notes"`
				Authors int    `json:"authors"`
			} `json:"monthly"`
		} `json:"sense_notes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatal(err)
	}
	sn := stats.SenseNotes
	if sn.Total != 53 || sn.Authors != 17 || sn.NonAdminAuthors != 16 || sn.Profiles != 18 || sn.NonAdminProfiles != 17 {
		t.Fatalf("overview %+v", sn)
	}
	if sn.FirstDay == nil || *sn.FirstDay != "2026-05-01" || sn.LastDay == nil || *sn.LastDay != "2026-06-29" {
		t.Fatalf("days %v %v", sn.FirstDay, sn.LastDay)
	}
	if sn.Buckets != struct {
		One      int `json:"one"`
		TwoFour  int `json:"two_four"`
		FiveNine int `json:"five_nine"`
		TenPlus  int `json:"ten_plus"`
	}{One: 3, TwoFour: 11, FiveNine: 2, TenPlus: 1} {
		t.Fatalf("buckets %+v", sn.Buckets)
	}
	if sn.ActiveDays.Days2Plus != 12 || sn.ActiveDays.Days3Plus != 5 || sn.ActiveDays.Days7Plus != 2 {
		t.Fatalf("active days %+v", sn.ActiveDays)
	}
	if sn.Repeat7.Total != 13 || sn.Repeat7.Repeat != 7 {
		t.Fatalf("repeat7 %+v", sn.Repeat7)
	}
	if sn.Repeat28.Total != 3 || sn.Repeat28.Repeat != 3 {
		t.Fatalf("repeat28 %+v", sn.Repeat28)
	}
	cohortByWeek := map[string][3]int{}
	for _, ch := range sn.Cohorts {
		cohortByWeek[ch.WeekStart] = [3]int{ch.Authors, ch.NextWeek, ch.SinceNextWeek}
	}
	wantCohorts := map[string][3]int{
		"2026-06-22": {2, 0, 0},
		"2026-06-15": {4, 2, 3},
		"2026-06-08": {3, 0, 0},
		"2026-06-01": {5, 2, 2},
		"2026-05-25": {1, 0, 1},
		"2026-05-18": {1, 1, 1},
		"2026-04-27": {1, 1, 1},
	}
	if len(cohortByWeek) != len(wantCohorts) {
		t.Fatalf("cohorts %+v", sn.Cohorts)
	}
	for week, want := range wantCohorts {
		if cohortByWeek[week] != want {
			t.Fatalf("cohort %s = %v, want %v", week, cohortByWeek[week], want)
		}
	}
	weeklyByWeek := map[string][2]int{}
	for _, w := range sn.Weekly {
		weeklyByWeek[w.WeekStart] = [2]int{w.Notes, w.Authors}
	}
	for week, want := range map[string][2]int{
		"2026-06-08": {14, 5}, "2026-06-01": {10, 5}, "2026-06-22": {6, 5},
		"2026-06-15": {4, 4}, "2026-05-04": {7, 1}, "2026-04-27": {3, 1},
	} {
		if weeklyByWeek[week] != want {
			t.Fatalf("weekly %s = %v, want %v", week, weeklyByWeek[week], want)
		}
	}
	if len(sn.Weekly) != 10 {
		t.Fatalf("weekly len %d", len(sn.Weekly))
	}
	monthlyByMonth := map[string][2]int{}
	for _, mo := range sn.Monthly {
		monthlyByMonth[mo.Month] = [2]int{mo.Notes, mo.Authors}
	}
	if monthlyByMonth["2026-05-01"] != [2]int{18, 3} || monthlyByMonth["2026-06-01"] != [2]int{35, 15} || len(sn.Monthly) != 2 {
		t.Fatalf("monthly %+v", sn.Monthly)
	}
	// 既存の統計はそのまま残る。
	if len(stats.Weeks) != 12 {
		t.Fatalf("weeks %d", len(stats.Weeks))
	}
	// 本文やアカウントは返さない。
	for _, secret := range []string{"秘密の感覚メモ本文", adminID.String(), a.String()} {
		if strings.Contains(rec.Body.String(), secret) {
			t.Fatalf("stats leaked content: %s", secret)
		}
	}
}

// 直近に始めた人だけのとき、28日の分母はゼロになり、日付は null、内訳は空になる。
func TestAdminSenseNoteStatsRepeatZero(t *testing.T) {
	fixed := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	h := setupConfig(t, func(cfg *config.Config) {
		cfg.Clock = func() time.Time { return fixed }
	})
	adminID := uuid.New()
	admin := token(t, adminID)
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", admin, nil); rec.Code != http.StatusOK {
		t.Fatalf("admin profile %d", rec.Code)
	}
	setAdmin(t, adminID)
	for _, day := range []string{"2026-06-23", "2026-06-24"} {
		id := uuid.New()
		if rec := doJSON(t, h, http.MethodGet, "/v1/me", token(t, id), nil); rec.Code != http.StatusOK {
			t.Fatalf("profile %d", rec.Code)
		}
		execSQLArgs(t, `INSERT INTO sense_notes (id, author_id, body, day) VALUES (gen_random_uuid(), $1, 'recent', $2::date)`, id, day)
	}

	rec := doJSON(t, h, http.MethodGet, "/v1/admin/stats", admin, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("stats %d %s", rec.Code, rec.Body.String())
	}
	var stats struct {
		SenseNotes struct {
			Total    int                         `json:"total"`
			FirstDay *string                     `json:"first_day"`
			LastDay  *string                     `json:"last_day"`
			Repeat7  struct{ Total, Repeat int } `json:"repeat_7"`
			Repeat28 struct{ Total, Repeat int } `json:"repeat_28"`
			Cohorts  []any                       `json:"cohorts"`
			Weekly   []any                       `json:"weekly"`
			Monthly  []any                       `json:"monthly"`
			Buckets  struct {
				One      int `json:"one"`
				TwoFour  int `json:"two_four"`
				FiveNine int `json:"five_nine"`
				TenPlus  int `json:"ten_plus"`
			} `json:"buckets"`
		} `json:"sense_notes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatal(err)
	}
	sn := stats.SenseNotes
	if sn.Total != 2 {
		t.Fatalf("total %d", sn.Total)
	}
	if sn.Repeat7.Total != 0 || sn.Repeat7.Repeat != 0 || sn.Repeat28.Total != 0 || sn.Repeat28.Repeat != 0 {
		t.Fatalf("repeat %+v %+v", sn.Repeat7, sn.Repeat28)
	}
	if sn.FirstDay == nil || *sn.FirstDay != "2026-06-23" || sn.LastDay == nil || *sn.LastDay != "2026-06-24" {
		t.Fatalf("days %v %v", sn.FirstDay, sn.LastDay)
	}
	if len(sn.Cohorts) != 1 || len(sn.Weekly) != 1 || len(sn.Monthly) != 1 {
		t.Fatalf("cohorts %d weekly %d monthly %d", len(sn.Cohorts), len(sn.Weekly), len(sn.Monthly))
	}
	if sn.Buckets != struct {
		One      int `json:"one"`
		TwoFour  int `json:"two_four"`
		FiveNine int `json:"five_nine"`
		TenPlus  int `json:"ten_plus"`
	}{One: 2} {
		t.Fatalf("buckets %+v", sn.Buckets)
	}
}
