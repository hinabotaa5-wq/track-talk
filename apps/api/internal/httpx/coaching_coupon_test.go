package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

// grantTestCoupon は、利用者に ¥500 クーポンを1枚付与する（付与の経路は紹介のテストで確かめる）。
func grantTestCoupon(t *testing.T, owner uuid.UUID, expires string) string {
	t.Helper()
	id := uuid.NewString()
	execSQLArgs(t, `INSERT INTO coaching_coupons (id, owner_id, source, amount_yen, expires_at)
		VALUES ($1, $2, 'referral', 500, now() + $3::interval)`, id, owner, expires)
	return id
}

func couponStatus(t *testing.T, id string) string {
	t.Helper()
	var status string
	queryRowArgs(t, `SELECT status FROM coaching_coupons WHERE id=$1`, []any{id}, &status)
	return status
}

// couponReview は、クーポンを使ってカードで申し込み、作成された依頼の ID を返す。
func couponReview(t *testing.T, h http.Handler, tok string, extra map[string]any) (string, int, string) {
	t.Helper()
	body := reviewBodyFor(t, h, tok)
	body["method"], body["use_coupon"] = "card", true
	for k, v := range extra {
		body[k] = v
	}
	rec := createReviewWith(t, h, tok, "card", body)
	var created reviewRequestView
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	return created.ID, rec.Code, rec.Body.String()
}

func authorizeReview(t *testing.T, h http.Handler, id, intent string) {
	t.Helper()
	var session string
	queryRowArgs(t, `SELECT checkout_session_id FROM review_payments WHERE request_id=$1`, []any{id}, &session)
	if rec := stripeWebhook(t, h, testWebhookSecret, "checkout.session.completed", map[string]any{
		"id": session, "payment_status": "unpaid", "payment_intent": intent, "metadata": map[string]any{"kind": "review"},
	}); rec.Code != http.StatusOK {
		t.Fatalf("authorize %d %s", rec.Code, rec.Body.String())
	}
}

// 本体 #1031 C1・C3：¥1,000 にクーポンを使うと支払いは ¥500。返却でクーポンは使用済になり、
// コーチ報酬は割引前の ¥1,000 × 70% = ¥700、運営負担の ¥500 は選手の支払いと分けて記録される。
func TestReviewCouponPaysRemainderAndKeepsEarnings(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	userID, tu := demoRequester(t, h, admin)
	coupon := grantTestCoupon(t, userID, "6 months")

	if rec := doJSON(t, h, http.MethodGet, "/v1/me/coaching/tickets", tu, nil); rec.Code != 200 ||
		!strings.Contains(rec.Body.String(), `"coupons":{"amount_yen":500,"available":1`) {
		t.Fatalf("coupon balance %d %s", rec.Code, rec.Body.String())
	}
	id, code, body := couponReview(t, h, tu, nil)
	if code != http.StatusCreated {
		t.Fatalf("create %d %s", code, body)
	}
	if !strings.Contains(body, `"cash_price_yen":500`) || !strings.Contains(body, `"coupon_discount_yen":500`) {
		t.Fatalf("created %s", body)
	}
	if form := f.lastSession(); form.Get("line_items[0][price_data][unit_amount]") != "500" {
		t.Fatalf("checkout amount %v", form)
	}
	if s := couponStatus(t, coupon); s != "reserved" {
		t.Fatalf("coupon after create %s", s)
	}
	authorizeReview(t, h, id, "pi_card_coupon1")
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+id+"/start", tc, nil); rec.Code != 200 {
		t.Fatalf("start %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", tc, responseBody()); rec.Code != 200 {
		t.Fatalf("return %d %s", rec.Code, rec.Body.String())
	}
	if captured, _ := f.calls(); len(captured) != 1 || captured[0] != "pi_card_coupon1" {
		t.Fatalf("captured %v", captured)
	}
	if s := couponStatus(t, coupon); s != "used" {
		t.Fatalf("coupon after return %s", s)
	}
	// 報酬は tick で記録する。
	if rec := tick(t, h, "tick-secret"); rec.Code != 200 {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
	var paid, gross, share, subsidy int
	queryRowArgs(t, `SELECT p.amount_yen, e.gross_yen, e.amount_yen, e.platform_subsidy_yen FROM coach_earnings e
		JOIN review_payments p ON p.request_id=e.review_request_id AND p.status='captured'
		WHERE e.review_request_id=$1 AND e.coach_id=$2`, []any{id, coach}, &paid, &gross, &share, &subsidy)
	if paid != 500 || gross != 1000 || share != 700 || subsidy != 500 {
		t.Fatalf("paid=%d gross=%d earnings=%d subsidy=%d", paid, gross, share, subsidy)
	}

	// C3：クーポンを使わない申込はこれまでどおり ¥1,000 を払い、割引も運営負担も記録しない。
	_, other := demoRequester(t, h, admin)
	plain := paidReview(t, h, other, "card", "pi_card_plain")
	if form := f.lastSession(); form.Get("line_items[0][price_data][unit_amount]") != "1000" {
		t.Fatalf("plain checkout %v", form)
	}
	var discount int
	queryRowArgs(t, `SELECT coupon_discount_yen FROM review_requests WHERE id=$1`, []any{plain}, &discount)
	if discount != 0 {
		t.Fatalf("plain discount %d", discount)
	}
}

// 本体 #1031 C4・C5：チケットとの併用、他人の・期限切れの・使用済のクーポンは使えない。断ったときは依頼もクーポンも残さない。
func TestReviewCouponRejectsInvalidUse(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	onDutyCoach(t, h, admin)
	userID, tu := demoRequester(t, h, admin)
	otherID, _ := demoRequester(t, h, admin)
	mine := grantTestCoupon(t, userID, "6 months")
	theirs := grantTestCoupon(t, otherID, "6 months")
	expired := grantTestCoupon(t, userID, "-1 minute")
	used := grantTestCoupon(t, userID, "6 months")
	execSQLArgs(t, `UPDATE coaching_coupons SET status='used', used_at=now() WHERE id=$1`, used)

	body := reviewBodyFor(t, h, tu)
	body["method"], body["use_coupon"] = "tickets", true
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, body); rec.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(rec.Body.String(), `"use_coupon"`) {
		t.Fatalf("coupon with tickets %d %s", rec.Code, rec.Body.String())
	}
	for name, id := range map[string]string{"other's": theirs, "expired": expired, "used": used} {
		if _, code, body := couponReview(t, h, tu, map[string]any{"coupon_id": id}); code != http.StatusUnprocessableEntity ||
			!strings.Contains(body, "coupon_unavailable") {
			t.Fatalf("%s coupon %d %s", name, code, body)
		}
	}
	var requests int
	queryRowArgs(t, `SELECT count(*) FROM review_requests WHERE requester_id=$1`, []any{userID}, &requests)
	if requests != 0 {
		t.Fatalf("rejected requests were kept: %d", requests)
	}
	for id, want := range map[string]string{mine: "unused", theirs: "unused", used: "used", expired: "unused"} {
		if s := couponStatus(t, id); s != want {
			t.Fatalf("coupon %s is %s, want %s", id, s, want)
		}
	}
	// 1回の申込に使えるのは1枚だけ。
	id, code, resp := couponReview(t, h, tu, nil)
	if code != http.StatusCreated {
		t.Fatalf("valid coupon %d %s", code, resp)
	}
	var reserved int
	queryRowArgs(t, `SELECT count(*) FROM coaching_coupons WHERE request_id=$1`, []any{id}, &reserved)
	if reserved != 1 || couponStatus(t, mine) != "reserved" {
		t.Fatalf("reserved %d, mine %s", reserved, couponStatus(t, mine))
	}
}

// 本体 #1031 C6：着手前のキャンセルと期限切れでは、チケットと同じく tick でクーポンが未使用に戻る。
// 戻す時点で有効期限を過ぎていれば、選手都合の終了では戻さずに失効させ、出し直さない。
func TestReviewCouponReleasedOnCancelAndTimeout(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	onDutyCoach(t, h, admin)
	userID, tu := demoRequester(t, h, admin)

	start := func(intent string) (string, string) {
		t.Helper()
		coupon := grantTestCoupon(t, userID, "6 months")
		id, code, body := couponReview(t, h, tu, map[string]any{"coupon_id": coupon})
		if code != http.StatusCreated {
			t.Fatalf("create %d %s", code, body)
		}
		authorizeReview(t, h, id, intent)
		return id, coupon
	}
	cancel := func(id string) {
		t.Helper()
		if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/cancel", tu, nil); rec.Code != 200 {
			t.Fatalf("cancel %d %s", rec.Code, rec.Body.String())
		}
	}
	canceled, canceledCoupon := start("pi_card_coupon_cancel")
	cancel(canceled)
	late, lateCoupon := start("pi_card_coupon_late")
	cancel(late)
	execSQLArgs(t, `UPDATE coaching_coupons SET expires_at=now()-interval '1 minute' WHERE id=$1`, lateCoupon)
	timedOut, timedOutCoupon := start("pi_card_coupon_timeout")
	execSQLArgs(t, `UPDATE review_requests SET due_at=now()-interval '1 minute' WHERE id=$1`, timedOut)

	if rec := tick(t, h, "tick-secret"); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"coupons_released":3`) {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
	if rec := tick(t, h, "tick-secret"); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"coupons_released":0`) {
		t.Fatalf("repeat tick %s", rec.Body.String())
	}
	for _, id := range []string{canceledCoupon, timedOutCoupon} {
		var status string
		var request *string
		queryRowArgs(t, `SELECT status, request_id::text FROM coaching_coupons WHERE id=$1`, []any{id}, &status, &request)
		if status != "unused" || request != nil {
			t.Fatalf("released coupon %s: %s %v", id, status, request)
		}
	}
	var total int
	queryRowArgs(t, `SELECT count(*) FROM coaching_coupons WHERE owner_id=$1`, []any{userID}, &total)
	if s := couponStatus(t, lateCoupon); s != "expired" || total != 3 {
		t.Fatalf("late coupon %s, coupons %d", s, total)
	}
}

// 本体 #1031 C2・C7：価格の全額をクーポンで割り引いた（支払い0円の）依頼は、支払いなしで返却まで進み、
// コーチ報酬は割引前の価格 × 70%（¥500 なら ¥350）、運営負担は価格の全額として記録する。
func TestReviewCouponFullDiscountRecordsEarnings(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setupConfig(t, func(cfg *config.Config) { cfg.CoachingTickSecret = "tick-secret" })
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	userID, tu := demoRequester(t, h, admin)
	coupon := grantTestCoupon(t, userID, "6 months")
	id, code, body := couponReview(t, h, tu, nil)
	if code != http.StatusCreated {
		t.Fatalf("create %d %s", code, body)
	}
	// 価格 ¥500 の依頼にクーポン ¥500 を使った状態にする（現行価格は ¥1,000 のため、行を直接合わせる）。
	execSQLArgs(t, `UPDATE review_requests SET base_price_yen=500 WHERE id=$1`, id)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+id+"/start", tc, nil); rec.Code != 200 {
		t.Fatalf("start %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", tc, responseBody()); rec.Code != 200 {
		t.Fatalf("return %d %s", rec.Code, rec.Body.String())
	}
	if s := couponStatus(t, coupon); s != "used" {
		t.Fatalf("coupon %s", s)
	}
	if rec := tick(t, h, "tick-secret"); rec.Code != 200 {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
	var gross, share, subsidy, payments int
	queryRowArgs(t, `SELECT gross_yen, amount_yen, platform_subsidy_yen FROM coach_earnings WHERE review_request_id=$1 AND coach_id=$2`,
		[]any{id, coach}, &gross, &share, &subsidy)
	queryRowArgs(t, `SELECT count(*) FROM review_payments WHERE request_id=$1`, []any{id}, &payments)
	if gross != 500 || share != 350 || subsidy != 500 || payments != 0 {
		t.Fatalf("gross=%d earnings=%d subsidy=%d payments=%d", gross, share, subsidy, payments)
	}
}
