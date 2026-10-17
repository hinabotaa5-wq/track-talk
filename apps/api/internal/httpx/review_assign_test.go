package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

type coachRequestsView struct {
	Offers []struct {
		ID      string            `json:"id"`
		Request reviewRequestView `json:"request"`
	} `json:"offers"`
	Reviewing []reviewRequestView `json:"reviewing"`
}

// onDutyCoach は、承認済みで受付中のコーチ（枠はコーチ、種目は110mハードルを含む）を作る。
func onDutyCoach(t *testing.T, h http.Handler, tadmin string) (uuid.UUID, string) {
	t.Helper()
	id := uuid.New()
	tc := token(t, id)
	body := coachApplicationBody()
	body["tier"] = "coach"
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", tc, body); rec.Code != http.StatusOK {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	withCoachPhoto(t, id)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+id.String()+"/approve", tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("approve %d", rec.Code)
	}
	execSQLArgs(t, `INSERT INTO push_subscriptions (id, profile_id, endpoint, p256dh, auth) VALUES ($1, $2, $3, 'key', 'auth')`,
		uuid.New(), id, "https://push.example.com/"+uuid.NewString())
	// 支払いのあるテストでも受付を始められるように、受け取り口座の登録を済ませておく。
	execSQLArgs(t, `UPDATE coach_profiles SET stripe_account_id = $2, payouts_enabled = true WHERE profile_id = $1`, id, "acct_"+id.String())
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", tc, map[string]any{"on_duty": true}); rec.Code != http.StatusOK {
		t.Fatalf("on duty %d %s", rec.Code, rec.Body.String())
	}
	return id, tc
}

func coachRequests(t *testing.T, h http.Handler, tc string) coachRequestsView {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/coach/requests", tc, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("coach requests %d %s", rec.Code, rec.Body.String())
	}
	var out coachRequestsView
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

func reviewStatus(t *testing.T, h http.Handler, tu, id string) string {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/coaching/requests/"+id, tu, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get %d %s", rec.Code, rec.Body.String())
	}
	var out reviewRequestView
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out.Status
}

func tick(t *testing.T, h http.Handler, secret string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/internal/coaching/tick", nil)
	if secret != "" {
		req.Header.Set("X-Coaching-Tick-Secret", secret)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestReviewOfferFlow(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setupConfig(t, func(cfg *config.Config) { cfg.CoachingTickSecret = "tick-secret" })
	tadmin := adminToken(t, h)
	_, ta := onDutyCoach(t, h, tadmin)
	_, tb := onDutyCoach(t, h, tadmin)

	user := uuid.New()
	tu := token(t, user)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/demo", tu, map[string]any{"code": newDemoCodeFor(t, h, tadmin)}); rec.Code != http.StatusOK {
		t.Fatalf("join %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPut, "/v1/me/birth-month", tu, map[string]any{"birth_month": "1995-04"}); rec.Code != http.StatusOK {
		t.Fatalf("birth month %d", rec.Code)
	}

	// 目安はいつでも翌日の12時。受付中のコーチがいるかどうかも返す。
	rec := doJSON(t, h, http.MethodGet, "/v1/coaching/estimate?tier=coach&event=100m", tu, nil)
	var est struct {
		DueAt         time.Time `json:"due_at"`
		CoachesOnDuty bool      `json:"coaches_on_duty"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &est)
	if rec.Code != http.StatusOK || !est.CoachesOnDuty || !est.DueAt.Equal(tomorrowNoon()) {
		t.Fatalf("estimate %d %s", rec.Code, rec.Body.String())
	}
	// いまは100mだけ受け付ける。
	for _, ev := range []string{"110mh", "marathon"} {
		if rec := doJSON(t, h, http.MethodGet, "/v1/coaching/estimate?tier=coach&event="+ev, tu, nil); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("estimate %s %d", ev, rec.Code)
		}
	}

	// 送るとすぐ、先に受付を始めたコーチ A に打診される。動画は引き受けるまで見せない。
	rec = doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, reviewBody())
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var first reviewRequestView
	_ = json.Unmarshal(rec.Body.Bytes(), &first)
	if first.Status != "offered" {
		t.Fatalf("first %+v", first)
	}
	offersA := coachRequests(t, h, ta)
	if len(offersA.Offers) != 1 || offersA.Offers[0].Request.ID != first.ID || offersA.Offers[0].Request.VideoURL != "" {
		t.Fatalf("A offers %+v", offersA)
	}
	if len(coachRequests(t, h, tb).Offers) != 0 {
		t.Fatal("B should have no offer yet")
	}
	// 他のコーチの打診は引き受けられない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/offers/"+offersA.Offers[0].ID+"/accept", tb, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("B accepts A's offer %d", rec.Code)
	}

	// A が見送ると B に回る。
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/offers/"+offersA.Offers[0].ID+"/decline", ta, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("decline %d %s", rec.Code, rec.Body.String())
	}
	offersB := coachRequests(t, h, tb)
	if len(offersB.Offers) != 1 || offersB.Offers[0].Request.ID != first.ID {
		t.Fatalf("B offers %+v", offersB)
	}

	// B が30分返事をしないと時間切れ。もう打診できる人がいないので待ちに戻る。
	execSQLArgs(t, `UPDATE review_offers SET expires_at = now() - interval '1 minute' WHERE result IS NULL`)
	if rec := tick(t, h, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("tick without secret %d", rec.Code)
	}
	if rec := tick(t, h, "wrong"); rec.Code != http.StatusNotFound {
		t.Fatalf("tick wrong secret %d", rec.Code)
	}
	rec = tick(t, h, "tick-secret")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"requeued":1`) || !strings.Contains(rec.Body.String(), `"offered":0`) {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/offers/"+offersB.Offers[0].ID+"/accept", tb, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("accept expired %d", rec.Code)
	}
	if got := reviewStatus(t, h, tu, first.ID); got != "queued" {
		t.Fatalf("after expiry %s", got)
	}

	// 2本目は、最後に打診されたのが古い A に回る。A が引き受けると動画が見られる。
	rec = doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, reviewBody())
	var second reviewRequestView
	_ = json.Unmarshal(rec.Body.Bytes(), &second)
	if rec.Code != http.StatusCreated || second.Status != "offered" {
		t.Fatalf("second %d %s", rec.Code, rec.Body.String())
	}
	offersA = coachRequests(t, h, ta)
	if len(offersA.Offers) != 1 || offersA.Offers[0].Request.ID != second.ID {
		t.Fatalf("A second offers %+v", offersA)
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/coach/offers/"+offersA.Offers[0].ID+"/accept", ta, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"in_review"`) || !strings.Contains(rec.Body.String(), "/video/authenticated/") {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}
	reviewing := coachRequests(t, h, ta).Reviewing
	if len(reviewing) != 1 || reviewing[0].ID != second.ID || reviewing[0].VideoURL == "" {
		t.Fatalf("reviewing %+v", reviewing)
	}
	if got := reviewStatus(t, h, tu, second.ID); got != "in_review" {
		t.Fatalf("requester sees %s", got)
	}

	// 返却の目安を過ぎた依頼は、tick を待たずに開いた時点で expired になり、通知が届く。
	execSQLArgs(t, `UPDATE review_requests SET due_at = now() - interval '1 minute'`)
	if got := reviewStatus(t, h, tu, second.ID); got != "expired" {
		t.Fatalf("overdue %s", got)
	}
	if got := reviewStatus(t, h, tu, first.ID); got != "expired" {
		t.Fatalf("overdue queued %s", got)
	}
	if len(coachRequests(t, h, ta).Reviewing) != 0 {
		t.Fatal("expired request still reviewing")
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me/notifications", tu, nil)
	if rec.Code != http.StatusOK || strings.Count(rec.Body.String(), `"kind":"review_expired"`) != 2 || !strings.Contains(rec.Body.String(), `"review_request_id":"`+second.ID+`"`) {
		t.Fatalf("notifications %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me/notifications", ta, nil)
	if rec.Code != http.StatusOK || strings.Count(rec.Body.String(), `"kind":"review_offer"`) != 2 {
		t.Fatalf("coach notifications %d %s", rec.Code, rec.Body.String())
	}
}

// 1人が同時に持てるのは2本まで。3本目は他に人がいなければ待ちになり、受付を始めた人に回る。
func TestReviewOfferRespectsActiveLimit(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	tadmin := adminToken(t, h)
	_, ta := onDutyCoach(t, h, tadmin)

	tu := token(t, uuid.New())
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/demo", tu, map[string]any{"code": newDemoCodeFor(t, h, tadmin)}); rec.Code != http.StatusOK {
		t.Fatalf("join %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPut, "/v1/me/birth-month", tu, map[string]any{"birth_month": "1995-04"}); rec.Code != http.StatusOK {
		t.Fatalf("birth month %d", rec.Code)
	}
	statuses := []string{}
	for range 3 {
		rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, reviewBody())
		var rr reviewRequestView
		_ = json.Unmarshal(rec.Body.Bytes(), &rr)
		statuses = append(statuses, rr.Status)
	}
	if strings.Join(statuses, ",") != "offered,offered,queued" {
		t.Fatalf("statuses %v", statuses)
	}
	if len(coachRequests(t, h, ta).Offers) != 2 {
		t.Fatal("A should hold 2 offers")
	}
	_, tb := onDutyCoach(t, h, tadmin)
	if got := coachRequests(t, h, tb).Offers; len(got) != 1 {
		t.Fatalf("B offers on duty %+v", got)
	}
}

// tomorrowNoon は、日本時間の翌日12時。
func tomorrowNoon() time.Time {
	tokyo := time.FixedZone("Asia/Tokyo", 9*60*60)
	now := time.Now().In(tokyo)
	return time.Date(now.Year(), now.Month(), now.Day()+1, 12, 0, 0, 0, tokyo)
}
