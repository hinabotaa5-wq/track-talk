package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func availabilityDay(offset int) string {
	return time.Now().In(time.FixedZone("Asia/Tokyo", 9*60*60)).AddDate(0, 0, offset).Format("2006-01-02")
}
func availabilityBody(date, status, note string) map[string]any {
	return map[string]any{"days": []map[string]string{{"date": date, "status": status, "note": note}}}
}
func availabilityCoachID(t *testing.T, plan string) string {
	t.Helper()
	var id string
	queryRowArgs(t, `SELECT coach_id::text FROM coaching_plans WHERE id=$1`, []any{plan}, &id)
	return id
}

func TestCoachAvailabilityAuthorizationAndDates(t *testing.T) {
	h := setup(t)
	coach, plan := inPersonPlan(t, h)
	id := availabilityCoachID(t, plan)
	path := "/v1/coach/me/availability"
	today := availabilityDay(0)
	for _, tc := range []struct {
		date, status, note string
		code               int
	}{
		{today, "available", "午後のみ", 200},
		{availabilityDay(20), "consult", strings.Repeat("あ", 30), 200},
		{availabilityDay(-1), "available", "", 422},
		{availabilityDay(21), "available", "", 422},
		{"2026-02-30", "available", "", 422},
		{today, "invalid", "", 422},
		{today, "available", strings.Repeat("あ", 31), 422},
		{today, "available", "test@example.com", 422},
		{today, "", "メモだけ", 422},
	} {
		rec := doJSON(t, h, "PUT", path, coach, availabilityBody(tc.date, tc.status, tc.note))
		if rec.Code != tc.code {
			t.Fatalf("save %+v: %d %s", tc, rec.Code, rec.Body.String())
		}
	}
	// 不正な一括更新は、正常な日も保存しない。
	rec := doJSON(t, h, "PUT", path, coach, map[string]any{"days": []map[string]string{
		{"date": today, "status": "unavailable"}, {"date": availabilityDay(21), "status": "consult"},
	}})
	if rec.Code != 422 {
		t.Fatal(rec.Body.String())
	}
	rec = doJSON(t, h, "GET", "/v1/coaches/"+id+"/availability", "", nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "午後のみ") || strings.Contains(rec.Body.String(), "unavailable") {
		t.Fatal(rec.Body.String())
	}
	// ほかの一般利用者・審査待ちコーチには読み書きさせない。
	outsider := token(t, uuid.New())
	pending := token(t, uuid.New())
	if r := doJSON(t, h, "POST", "/v1/coach/applications", pending, coachApplicationBody()); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	for _, tok := range []string{outsider, pending} {
		for _, method := range []string{"GET", "PUT"} {
			r := doJSON(t, h, method, path, tok, availabilityBody(today, "unavailable", ""))
			if r.Code != 403 {
				t.Fatalf("unauthorized %s: %d %s", method, r.Code, r.Body.String())
			}
		}
	}
	// 別の承認済みコーチの更新は、その人自身のカレンダーだけに保存される。
	_, other, _ := planCoach(t, h, 3)
	if r := doJSON(t, h, "PUT", path, other, availabilityBody(today, "unavailable", "")); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	r := doJSON(t, h, "GET", path, coach, nil)
	if !strings.Contains(r.Body.String(), "午後のみ") {
		t.Fatal("another coach changed owner calendar")
	}
	// 過去の記録を取得せず、次の保存で掃除する。
	execSQLArgs(t, `INSERT INTO coach_availability(coach_id,day,status) VALUES($1,$2,'available')`, id, availabilityDay(-1))
	r = doJSON(t, h, "GET", path, coach, nil)
	if strings.Contains(r.Body.String(), availabilityDay(-1)) {
		t.Fatal("past day returned")
	}
	r = doJSON(t, h, "PUT", path, coach, availabilityBody(today, "", ""))
	if r.Code != 200 || strings.Contains(r.Body.String(), "午後のみ") {
		t.Fatal(r.Body.String())
	}
	var count int
	queryRowArgs(t, `SELECT count(*) FROM coach_availability WHERE coach_id=$1 AND day=$2`, []any{id, availabilityDay(-1)}, &count)
	if count != 0 {
		t.Fatal("past day retained after save")
	}
	// 承認取り消し後も保存できない。
	execSQLArgs(t, `UPDATE coach_profiles SET status='rejected' WHERE profile_id=$1`, id)
	if r := doJSON(t, h, "PUT", path, coach, availabilityBody(today, "available", "")); r.Code != 403 {
		t.Fatal(r.Body.String())
	}
	r = doJSON(t, h, "GET", "/v1/coaches/"+id+"/availability", "", nil)
	if !strings.Contains(r.Body.String(), `"days":[]`) {
		t.Fatal("revoked calendar exposed")
	}
}

func TestPublicAvailabilityRequiresEligiblePublishedPlan(t *testing.T) {
	h := setup(t)
	id, coach, plan := planCoach(t, h, 3)
	// 動画添削しか公開していないコーチの空き状況は出さない。
	execSQLArgs(t, `UPDATE coaching_plans SET service_type='video_review' WHERE id=$1`, plan)
	execSQLArgs(t, `UPDATE coach_profiles SET service_types=ARRAY['video_review'] WHERE profile_id=$1`, id)
	r := doJSON(t, h, "PUT", "/v1/coach/me/availability", coach, availabilityBody(availabilityDay(0), "available", "午前のみ"))
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	path := "/v1/coaches/" + id.String() + "/availability"
	assertEmpty := func() {
		t.Helper()
		r := doJSON(t, h, "GET", path, "", nil)
		if r.Code != 200 || !strings.Contains(r.Body.String(), `"days":[]`) {
			t.Fatal(r.Body.String())
		}
	}
	assertEmpty()
	execSQLArgs(t, `UPDATE coaching_plans SET service_type='in_person' WHERE id=$1`, plan)
	execSQLArgs(t, `UPDATE coach_profiles SET service_types=ARRAY['in_person'] WHERE profile_id=$1`, id)
	r = doJSON(t, h, "GET", path, "", nil)
	if !strings.Contains(r.Body.String(), "午前のみ") {
		t.Fatal(r.Body.String())
	}
	execSQLArgs(t, `UPDATE coaching_plans SET published=false WHERE id=$1`, plan)
	assertEmpty()
	execSQLArgs(t, `UPDATE coaching_plans SET published=true WHERE id=$1`, plan)
	execSQLArgs(t, `UPDATE coach_profiles SET pledge_version=0 WHERE profile_id=$1`, id)
	assertEmpty()
}

func TestCoachingPreferredDatesValidationAndSnapshot(t *testing.T) {
	for _, service := range []string{"in_person", "competition_support"} {
		t.Run(service, func(t *testing.T) {
			h := setup(t)
			var coach, plan string
			if service == "in_person" {
				coach, plan = inPersonPlan(t, h)
			} else {
				coach, plan = supportPlan(t, h)
			}
			for offset, status := range []string{"available", "consult", "unavailable", "available", "consult"} {
				if r := doJSON(t, h, "PUT", "/v1/coach/me/availability", coach, availabilityBody(availabilityDay(offset), status, "")); r.Code != 200 {
					t.Fatal(r.Body.String())
				}
			}
			_, user := demoUser(t, h, time.Date(1995, 1, 1, 0, 0, 0, 0, time.UTC))
			for _, dates := range [][]string{
				{availabilityDay(2)}, {availabilityDay(5)}, {availabilityDay(-1)}, {availabilityDay(21)}, {"bad-date"},
				{availabilityDay(0), availabilityDay(0)},
				{availabilityDay(0), availabilityDay(1), availabilityDay(3), availabilityDay(4)},
			} {
				body := contractBody(plan)
				body["preferred_dates"] = dates
				r := doJSON(t, h, "POST", "/v1/coaching/contracts", user, body)
				if r.Code != 422 || !strings.Contains(r.Body.String(), "preferred_dates") {
					t.Fatalf("dates %v: %d %s", dates, r.Code, r.Body.String())
				}
			}
			dates := []string{availabilityDay(0), availabilityDay(1), availabilityDay(3)}
			body := contractBody(plan)
			body["preferred_dates"] = dates
			r := doJSON(t, h, "POST", "/v1/coaching/contracts", user, body)
			if r.Code != 201 {
				t.Fatalf("valid dates: %d %s", r.Code, r.Body.String())
			}
			contract := decodeContract(t, r.Body.Bytes())
			// 後から空き状況が変わっても、申し込んだ希望日は残る。
			doJSON(t, h, "PUT", "/v1/coach/me/availability", coach, availabilityBody(dates[0], "unavailable", ""))
			for _, req := range []struct{ path, token string }{
				{"/v1/coaching/contracts/" + contract.ID, user}, {"/v1/coaching/contracts/" + contract.ID, coach},
				{"/v1/coach/contracts", coach}, {"/v1/me/coaching/contracts", user},
			} {
				r := doJSON(t, h, "GET", req.path, req.token, nil)
				encoded, _ := json.Marshal(dates)
				if r.Code != 200 || !strings.Contains(r.Body.String(), `"preferred_dates":`+string(encoded)) {
					t.Fatalf("snapshot %s: %d %s", req.path, r.Code, r.Body.String())
				}
			}
		})
	}
}

func TestVideoReviewRejectsPreferredDates(t *testing.T) {
	h := setup(t)
	id, _, plan := planCoach(t, h, 3)
	execSQLArgs(t, `UPDATE coach_profiles SET service_types=ARRAY['video_review'] WHERE profile_id=$1`, id)
	execSQLArgs(t, `UPDATE coaching_plans SET service_type='video_review' WHERE id=$1`, plan)
	_, user := demoUser(t, h, time.Date(1995, 1, 1, 0, 0, 0, 0, time.UTC))
	body := contractBody(plan)
	body["preferred_dates"] = []string{availabilityDay(0)}
	r := doJSON(t, h, "POST", "/v1/coaching/contracts", user, body)
	if r.Code != http.StatusUnprocessableEntity {
		t.Fatalf("video preferred dates: %d %s", r.Code, r.Body.String())
	}
	// 希望日なしの既存の申し込みを妨げない。
	r = doJSON(t, h, "POST", "/v1/coaching/contracts", user, contractBody(plan))
	if r.Code != 201 || !strings.Contains(r.Body.String(), `"preferred_dates":[]`) {
		t.Fatal(r.Body.String())
	}
}
