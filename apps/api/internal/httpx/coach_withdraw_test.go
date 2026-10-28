package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

const coachWithdrawPath = "/v1/coach/me/withdraw"

func withdrawCoachBody() map[string]any {
	return map[string]any{"confirm": "コーチをやめる"}
}

// コーチだけをやめても、アカウントは残る。コーチ一覧から外れ、もう一度応募すれば審査からやり直す。
func TestCoachWithdrawKeepsAccount(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	tadmin := adminToken(t, h)
	id, tc := onDutyCoach(t, h, tadmin)

	if rec := doJSON(t, h, http.MethodPost, coachWithdrawPath, tc, map[string]any{"confirm": "退会する"}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("wrong confirm %d %s", rec.Code, rec.Body.String())
	}
	rec := doJSON(t, h, http.MethodPost, coachWithdrawPath, tc, withdrawCoachBody())
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"withdrawn"`) {
		t.Fatalf("withdraw %d %s", rec.Code, rec.Body.String())
	}
	var onDuty bool
	var photo *string
	queryRowArgs(t, `SELECT on_duty, photo_public_id FROM coach_profiles WHERE profile_id = $1`, []any{id}, &onDuty, &photo)
	if onDuty || photo != nil {
		t.Fatalf("after withdraw on_duty=%v photo=%v", onDuty, photo)
	}
	if body := doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil).Body.String(); strings.Contains(body, id.String()) {
		t.Fatalf("withdrawn coach still listed %s", body)
	}
	// アカウントは残り、ふつうに使える。
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", tc, nil); rec.Code != http.StatusOK {
		t.Fatalf("me after withdraw %d", rec.Code)
	}
	// コーチの機能は使えなくなる。
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", tc, map[string]any{"on_duty": true}); rec.Code == http.StatusOK {
		t.Fatalf("withdrawn coach turned on duty %s", rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, coachWithdrawPath, tc, withdrawCoachBody()); rec.Code != http.StatusConflict {
		t.Fatalf("withdraw twice %d %s", rec.Code, rec.Body.String())
	}
	// もう一度応募すると、審査待ちに戻る。
	rec = doJSON(t, h, http.MethodPost, "/v1/coach/applications", tc, coachApplicationBody())
	var me coachMeView
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if rec.Code != http.StatusOK || me.Application == nil || me.Application.Status != "pending" || me.Application.ReviewedAt != nil {
		t.Fatalf("reapply %d %s", rec.Code, rec.Body.String())
	}
}

// 審査待ちの応募も取り下げられる。応募していなければ何もしない。
func TestCoachWithdrawPendingApplication(t *testing.T) {
	h := setup(t)
	_, tu := demoUser(t, h, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
	if rec := doJSON(t, h, http.MethodPost, coachWithdrawPath, tu, withdrawCoachBody()); rec.Code != http.StatusConflict {
		t.Fatalf("no application %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", tu, coachApplicationBody()); rec.Code != http.StatusOK {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, coachWithdrawPath, tu, withdrawCoachBody()); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"withdrawn"`) {
		t.Fatalf("withdraw pending %d %s", rec.Code, rec.Body.String())
	}
}

// 進行中のパーソナル指導があるあいだは、コーチをやめられない。
func TestCoachWithdrawBlockedByOpenContract(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	tc, _, _ := activeContract(t, h)
	rec := doJSON(t, h, http.MethodPost, coachWithdrawPath, tc, withdrawCoachBody())
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "coaching_in_progress") {
		t.Fatalf("withdraw with contract %d %s", rec.Code, rec.Body.String())
	}
	if body := doJSON(t, h, http.MethodGet, "/v1/coach/me", tc, nil).Body.String(); !strings.Contains(body, `"status":"approved"`) {
		t.Fatalf("status changed %s", body)
	}
}

// 添削を引き受けたあとは、返却するまでコーチをやめられない。
func TestCoachWithdrawBlockedByReviewing(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	tadmin := adminToken(t, h)
	_, tc := onDutyCoach(t, h, tadmin)
	_, tu := demoRequester(t, h, tadmin)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, reviewBodyFor(t, h, tu)); rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	offers := coachRequests(t, h, tc).Offers
	if len(offers) != 1 {
		t.Fatalf("offers %+v", offers)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/offers/"+offers[0].ID+"/accept", tc, nil); rec.Code != http.StatusOK {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}
	rec := doJSON(t, h, http.MethodPost, coachWithdrawPath, tc, withdrawCoachBody())
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "review_in_progress") {
		t.Fatalf("withdraw while reviewing %d %s", rec.Code, rec.Body.String())
	}
}

// 返事をしていない添削の打診は断った扱いになり、依頼は待ちに戻ってほかのコーチに回る。
func TestCoachWithdrawHandsOffOpenOffers(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	tadmin := adminToken(t, h)
	_, tc := onDutyCoach(t, h, tadmin)
	_, tu := demoRequester(t, h, tadmin)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, reviewBodyFor(t, h, tu)); rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	offers := coachRequests(t, h, tc).Offers
	if len(offers) != 1 {
		t.Fatalf("offers %+v", offers)
	}
	if rec := doJSON(t, h, http.MethodPost, coachWithdrawPath, tc, withdrawCoachBody()); rec.Code != http.StatusOK {
		t.Fatalf("withdraw with offer %d %s", rec.Code, rec.Body.String())
	}
	if status := reviewStatus(t, h, tu, offers[0].Request.ID); status != "queued" {
		t.Fatalf("request after withdraw %s", status)
	}
	var result string
	queryRowArgs(t, `SELECT result FROM review_offers WHERE id = $1`, []any{offers[0].ID}, &result)
	if result != "declined" {
		t.Fatalf("offer result %s", result)
	}
}

// 未送金の報酬があるのに受け取り口座がなければ、先に口座を登録してもらう。
func TestCoachWithdrawNeedsPayoutAccountForUnpaidEarnings(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	tadmin := adminToken(t, h)
	id, tc := onDutyCoach(t, h, tadmin)
	execSQLArgs(t, `UPDATE coach_profiles SET payouts_enabled = false, stripe_account_id = NULL WHERE profile_id = $1`, id)
	execSQLArgs(t, `INSERT INTO coach_earnings (id, coach_id, source, gross_yen, share_percent, amount_yen) VALUES (gen_random_uuid(), $1, 'review', 1000, 70, 700)`, id)
	rec := doJSON(t, h, http.MethodPost, coachWithdrawPath, tc, withdrawCoachBody())
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "payouts_not_ready") {
		t.Fatalf("withdraw without payout account %d %s", rec.Code, rec.Body.String())
	}
}
