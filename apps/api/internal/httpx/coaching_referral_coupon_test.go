package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestCoachingReferralCoupon(t *testing.T) {
	// Migrate one isolated database for the suite. setupStripe resets its data
	// before each case so every case still exercises an independent scenario.
	isolateReferralDatabase(t)
	fakeCoachingCloudinary(t)
	for _, test := range []struct {
		name string
		run  func(*testing.T, http.Handler)
	}{
		{"purchase and used coupon withdrawal", coachingReferralCouponPurchaseAndWithdrawal},
		{"release after qualifying refund", coachingReferralCouponReleaseAfterQualifyingRefund},
		{"earliest expiry and concurrent use", coachingReferralCouponConcurrentUseAndEarliestExpiry},
		{"expired coupon reissue", coachingReferralCouponExpiredReissue},
		{"unused coupon and legacy lot withdrawal", coachingReferralUnusedCouponAndLegacyLotWithdrawal},
		{"backfill and guarded rollback", coachingReferralCouponMigrationBackfillsEarningLinks},
		{"review and resubmission deadlines", coachingReferralCouponRestoredAfterReviewDeadlines},
		{"standard ticket earning after withdrawal", ticketReviewEarningRetainsReservationAfterWithdrawal},
	} {
		t.Run(test.name, func(t *testing.T) {
			h, _ := setupStripe(t)
			test.run(t, h)
		})
	}
}

func grantReferralCoupon(t *testing.T, h http.Handler, admin string) (uuid.UUID, string, uuid.UUID, string) {
	t.Helper()
	ownerID, owner := demoRequester(t, h, admin)
	execSQLArgs(t, `SELECT coaching_issue_referral($1, 'purchase')`, ownerID)
	code := myReferral(t, h, owner).Code
	_, first := demoRequester(t, h, admin)
	_, second := demoRequester(t, h, admin)
	firstPack := buySixWithReferral(t, h, first, code)
	buySixWithReferral(t, h, second, code)
	var coupon uuid.UUID
	queryRowArgs(t, `SELECT coupon_id FROM coaching_referral_rewards WHERE referral_code=$1`, []any{code}, &coupon)
	return ownerID, owner, coupon, firstPack
}

func couponReviewBody() map[string]any {
	return map[string]any{"tier": "coach", "event": "100m", "question": "スタートの姿勢を見直したいです", "use_coupon": true, "guardian_permission": true}
}

func createCouponReview(t *testing.T, h http.Handler, owner string) string {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", owner, couponReviewBody())
	var result struct {
		ID         string `json:"id"`
		UsedCoupon bool   `json:"used_coupon"`
		UseTickets bool   `json:"use_tickets"`
		CashPrice  int    `json:"cash_price_yen"`
		Checkout   string `json:"checkout_url"`
	}
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &result) != nil ||
		result.ID == "" || !result.UsedCoupon || result.UseTickets || result.CashPrice != 500 || result.Checkout != "" {
		t.Fatalf("coupon create: %d %s", rec.Code, rec.Body.String())
	}
	return result.ID
}

func coachingReferralCouponPurchaseAndWithdrawal(t *testing.T, h http.Handler) {
	admin := adminToken(t, h)
	_, coach := onDutyCoach(t, h, admin)
	ownerID, owner, coupon, pack := grantReferralCoupon(t, h, admin)
	r := doJSON(t, h, http.MethodGet, "/v1/admin/coaching/referrals", admin, nil)
	if r.Code != 200 || !strings.Contains(r.Body.String(), myReferral(t, h, owner).Code) {
		t.Fatalf("admin referral overview: %d %s", r.Code, r.Body.String())
	}
	summary := doJSON(t, h, http.MethodGet, "/v1/me/coaching/tickets", owner, nil)
	var balance struct {
		Balance struct{ Available int }
		Coupon  struct {
			Count             int
			EarliestExpiresAt *string `json:"earliest_expires_at"`
		}
	}
	if summary.Code != 200 || json.Unmarshal(summary.Body.Bytes(), &balance) != nil ||
		balance.Balance.Available != 0 || balance.Coupon.Count != 1 || balance.Coupon.EarliestExpiresAt == nil {
		t.Fatalf("coupon summary: %d %s", summary.Code, summary.Body.String())
	}
	_, other := demoRequester(t, h, admin)
	if got := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", other, couponReviewBody()); got.Code != 422 || !strings.Contains(got.Body.String(), "coupon_unavailable") {
		t.Fatalf("non-owner: %d %s", got.Code, got.Body.String())
	}
	execSQLArgs(t, `UPDATE coaching_referral_coupons SET expires_at=now()-interval '1 hour' WHERE id=$1`, coupon)
	if got := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", owner, couponReviewBody()); got.Code != 422 || !strings.Contains(got.Body.String(), "coupon_unavailable") {
		t.Fatalf("expired coupon: %d %s", got.Code, got.Body.String())
	}
	execSQLArgs(t, `UPDATE coaching_referral_coupons SET expires_at=granted_at+interval '6 months' WHERE id=$1`, coupon)
	if got := doJSON(t, h, http.MethodPost, "/v1/coaching/tickets/checkout", owner, map[string]any{
		"product_id": "six", "request_key": uuid.NewString(), "guardian_permission": true, "use_coupon": true,
	}); got.Code != http.StatusUnprocessableEntity {
		t.Fatalf("coupon on ticket pack: %d %s", got.Code, got.Body.String())
	}
	missingPermission := couponReviewBody()
	delete(missingPermission, "guardian_permission")
	if got := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", owner, missingPermission); got.Code != 422 || !strings.Contains(got.Body.String(), "guardian_permission_required") {
		t.Fatalf("missing declaration: %d %s", got.Code, got.Body.String())
	}
	for field, value := range map[string]any{"method": "card", "use_tickets": true, "ticket_kind": "normal"} {
		incompatible := couponReviewBody()
		incompatible[field] = value
		if got := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", owner, incompatible); got.Code != 400 {
			t.Fatalf("incompatible %s: %d %s", field, got.Code, got.Body.String())
		}
	}
	id := createCouponReview(t, h, owner)
	for _, path := range []string{"/v1/coaching/requests/" + id, "/v1/me/coaching/requests"} {
		if got := doJSON(t, h, http.MethodGet, path, owner, nil); got.Code != 200 || !strings.Contains(got.Body.String(), `"used_coupon":true`) {
			t.Fatalf("coupon flag %s: %d %s", path, got.Code, got.Body.String())
		}
	}
	var payments, notices, allocations int
	queryRowArgs(t, `SELECT count(*) FROM review_payments WHERE request_id=$1`, []any{id}, &payments)
	queryRowArgs(t, `SELECT count(*) FROM minor_purchase_declarations WHERE ref_type='review' AND ref_id=$1 AND amount_yen=500`, []any{id}, &notices)
	queryRowArgs(t, `SELECT count(*) FROM coaching_ticket_allocations a JOIN coaching_ticket_reservations t ON t.id=a.reservation_id WHERE t.request_id=$1`, []any{id}, &allocations)
	if payments != 0 || notices != 1 || allocations != 0 || activeCodeCount(t, ownerID) != 0 {
		t.Fatalf("coupon accounting payments=%d notices=%d allocations=%d", payments, notices, allocations)
	}
	execSQLArgs(t, `UPDATE coaching_referral_coupons SET expires_at=now()-interval '1 day' WHERE id=$1`, coupon)
	if got := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+id+"/start", coach, nil); got.Code != 200 {
		t.Fatalf("start: %d %s", got.Code, got.Body.String())
	}
	if got := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", coach, responseBody()); got.Code != 200 {
		t.Fatalf("return: %d %s", got.Code, got.Body.String())
	}
	assertTicketAccounting(t, h, admin)
	var reservation uuid.UUID
	var amount int
	queryRowArgs(t, `SELECT e.reservation_id,e.amount_yen FROM coach_earnings e JOIN coaching_ticket_reservations t
        ON t.id=e.reservation_id WHERE e.review_request_id=$1 AND t.status='consumed'`, []any{id}, &reservation, &amount)
	if amount != 350 {
		t.Fatalf("coupon earning = %d, want 350", amount)
	}
	execSQLArgs(t, `UPDATE coaching_ticket_purchases SET refunded_yen=amount_yen,status='refunded' WHERE id=$1`, pack)
	var retained bool
	queryRowArgs(t, `SELECT used_reservation_id=$2 AND canceled_at IS NULL FROM coaching_referral_coupons WHERE id=$1`, []any{coupon, reservation}, &retained)
	if !retained {
		t.Fatal("refunding a count took back a consumed coupon")
	}
	execSQLArgs(t, `DELETE FROM profiles WHERE id=$1`, ownerID)
	queryRowArgs(t, `SELECT NOT EXISTS (SELECT FROM coaching_referral_coupons WHERE id=$1)
        AND NOT EXISTS (SELECT FROM coaching_referral_rewards WHERE coupon_id=$1)
        AND EXISTS (SELECT FROM coach_earnings e JOIN coaching_ticket_reservations t
            ON t.id=e.reservation_id WHERE t.id=$2 AND t.request_id IS NULL AND t.owner_id IS NULL AND e.amount_yen=350)`, []any{coupon, reservation}, &retained)
	if !retained {
		t.Fatal("withdrawal failed to remove coupon and retain linked accounting")
	}
}

func coachingReferralCouponReleaseAfterQualifyingRefund(t *testing.T, h http.Handler) {
	admin := adminToken(t, h)
	onDutyCoach(t, h, admin)
	ownerID, owner, coupon, pack := grantReferralCoupon(t, h, admin)
	id := createCouponReview(t, h, owner)
	if got := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/cancel", owner, nil); got.Code != 200 {
		t.Fatalf("cancel: %d %s", got.Code, got.Body.String())
	}
	if got := tick(t, h, "tick-secret"); got.Code != 200 {
		t.Fatalf("release tick: %d %s", got.Code, got.Body.String())
	}
	var restored bool
	queryRowArgs(t, `SELECT used_at IS NULL AND used_reservation_id IS NULL FROM coaching_referral_coupons WHERE id=$1`, []any{coupon}, &restored)
	if !restored {
		t.Fatal("coupon was not restored on normal cancellation")
	}
	id = createCouponReview(t, h, owner)
	execSQLArgs(t, `UPDATE coaching_ticket_purchases SET refunded_yen=amount_yen,status='refunded' WHERE id=$1`, pack)
	queryRowArgs(t, `SELECT cp.canceled_at IS NULL AND r.canceled_at IS NULL AND r.qualifying_refund_at IS NOT NULL
        FROM coaching_referral_coupons cp JOIN coaching_referral_rewards r ON r.coupon_id=cp.id WHERE cp.id=$1`, []any{coupon}, &restored)
	if !restored {
		t.Fatal("qualifying refund did not leave the in-flight coupon reserved")
	}
	if got := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/cancel", owner, nil); got.Code != 200 {
		t.Fatalf("cancel after refund: %d %s", got.Code, got.Body.String())
	}
	if got := tick(t, h, "tick-secret"); got.Code != 200 {
		t.Fatalf("refund release tick: %d %s", got.Code, got.Body.String())
	}
	queryRowArgs(t, `SELECT cp.canceled_at IS NOT NULL AND r.canceled_at IS NOT NULL
        FROM coaching_referral_coupons cp JOIN coaching_referral_rewards r ON r.coupon_id=cp.id WHERE cp.id=$1`, []any{coupon}, &restored)
	if !restored || activeCodeCount(t, ownerID) != 1 {
		t.Fatal("coupon returned after qualifying refund, or its cycle did not reopen")
	}
	execSQLArgs(t, `DELETE FROM profiles WHERE id=$1`, ownerID)
}

func coachingReferralCouponConcurrentUseAndEarliestExpiry(t *testing.T, h http.Handler) {
	admin := adminToken(t, h)
	ownerID, owner, earliest, _ := grantReferralCoupon(t, h, admin)
	later := uuid.New()
	execSQLArgs(t, `UPDATE coaching_referral_coupons SET expires_at=now()+interval '1 day' WHERE id=$1`, earliest)
	execSQLArgs(t, `INSERT INTO coaching_referral_coupons(id,owner_id,expires_at) VALUES ($1,$2,now()+interval '2 days')`, later, ownerID)
	createCouponReview(t, h, owner)
	var used, laterAvailable bool
	queryRowArgs(t, `SELECT used_at IS NOT NULL FROM coaching_referral_coupons WHERE id=$1`, []any{earliest}, &used)
	queryRowArgs(t, `SELECT used_at IS NULL FROM coaching_referral_coupons WHERE id=$1`, []any{later}, &laterAvailable)
	if !used || !laterAvailable {
		t.Fatal("first review did not use just the earliest coupon")
	}
	start := make(chan struct{})
	results := make(chan int, 2)
	var workers sync.WaitGroup
	for i := 0; i < 2; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", owner, couponReviewBody())
			if rec.Code != 201 && (rec.Code != 422 || !strings.Contains(rec.Body.String(), "coupon_unavailable")) {
				t.Errorf("concurrent checkout: %d %s", rec.Code, rec.Body.String())
			}
			results <- rec.Code
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	statuses := map[int]int{}
	for code := range results {
		statuses[code]++
	}
	var requests int
	queryRowArgs(t, `SELECT count(*) FROM review_requests WHERE requester_id=$1`, []any{ownerID}, &requests)
	if statuses[201] != 1 || statuses[422] != 1 || requests != 2 {
		t.Fatalf("concurrent statuses=%v requests=%d", statuses, requests)
	}
}

func coachingReferralCouponExpiredReissue(t *testing.T, h http.Handler) {
	admin := adminToken(t, h)
	_, coach := onDutyCoach(t, h, admin)
	ownerID, owner, coupon, _ := grantReferralCoupon(t, h, admin)
	id := createCouponReview(t, h, owner)
	execSQLArgs(t, `UPDATE coaching_referral_coupons SET expires_at=now()-interval '1 hour' WHERE id=$1`, coupon)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+id+"/withdraw", coach,
		map[string]any{"reason_code": "video_unviewable"}); rec.Code != 200 {
		t.Fatalf("withdraw %d %s", rec.Code, rec.Body.String())
	}
	for i := 0; i < 2; i++ {
		if rec := tick(t, h, "tick-secret"); rec.Code != 200 {
			t.Fatalf("release %d %s", rec.Code, rec.Body.String())
		}
	}
	var newCoupons, rewards int
	queryRowArgs(t, `SELECT count(*) FROM coaching_referral_coupons WHERE owner_id=$1 AND id<>$2
        AND expires_at=granted_at+interval '6 months' AND used_at IS NULL`, []any{ownerID, coupon}, &newCoupons)
	queryRowArgs(t, `SELECT count(*) FROM coaching_referral_rewards WHERE referrer_id=$1`, []any{ownerID}, &rewards)
	if newCoupons != 1 || rewards != 1 {
		t.Fatalf("reissued coupons=%d rewards=%d, want 1 and 1", newCoupons, rewards)
	}
	notices := 0
	for _, n := range coachingNotifications(t, h, owner) {
		if n.Title == "紹介特典：期限切れの無料クーポンを出し直しました" {
			notices++
			if n.URL != "/me/coaching/tickets" || n.Body != "運営者・コーチの都合で終わった1回プランで使った分を出し直しました。新しい特典クーポン：特典クーポン：付与日から6か月有効／付与された本人だけが次回の購入で使えます／払戻しはできません" {
				t.Fatalf("reissue notice %+v", n)
			}
		}
	}
	if notices != 1 {
		t.Fatalf("reissue notification count %d", notices)
	}
}

func coachingReferralUnusedCouponAndLegacyLotWithdrawal(t *testing.T, h http.Handler) {
	admin := adminToken(t, h)
	couponOwner, _, coupon, _ := grantReferralCoupon(t, h, admin)
	lotOwner, _, lot := grantRewardLot(t, h, admin)
	execSQLArgs(t, `DELETE FROM profiles WHERE id IN ($1,$2)`, couponOwner, lotOwner)
	var ok bool
	queryRowArgs(t, `SELECT NOT EXISTS (SELECT FROM coaching_referral_coupons WHERE id=$1)
        AND NOT EXISTS (SELECT FROM coaching_referral_rewards WHERE coupon_id=$1)
        AND EXISTS (SELECT FROM coaching_ticket_lots l JOIN coaching_referral_rewards r ON r.lot_id=l.id
            WHERE l.id=$2 AND l.owner_id IS NULL AND r.referrer_id IS NULL)`, []any{coupon, lot}, &ok)
	if !ok {
		t.Fatal("withdrawal did not remove coupon history or preserve legacy lot history")
	}
}

func coachingReferralCouponMigrationBackfillsEarningLinks(t *testing.T, h http.Handler) {
	admin := adminToken(t, h)
	owner, _ := demoRequester(t, h, admin)
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	migration := func(direction string) {
		t.Helper()
		source, err := os.ReadFile("../../migrations/000187_referral_coupon." + direction + ".sql")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, string(source)); err != nil {
			t.Fatalf("migration %s: %v", direction, err)
		}
	}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	migration("down")
	reservation, ticketReview, cashReview := uuid.New(), uuid.New(), uuid.New()
	for _, request := range []uuid.UUID{ticketReview, cashReview} {
		exec(`INSERT INTO review_requests(id,requester_id,tier,event,focus,question,status,due_at)
            VALUES ($1,$2,'coach','100m',ARRAY['start'],'Backfill test','returned',now())`, request, owner)
		exec(`INSERT INTO coach_earnings(id,source,review_request_id,gross_yen,share_percent,amount_yen,confirmed_at)
            VALUES (gen_random_uuid(),'review',$1,500,70,350,now())`, request)
	}
	exec(`INSERT INTO coaching_ticket_reservations(id,owner_id,request_id,quantity,nominal_yen,share_percent,status)
        VALUES ($1,$2,$3,1,500,70,'consumed')`, reservation, owner, ticketReview)
	migration("up")
	for _, test := range []struct {
		request uuid.UUID
		want    *uuid.UUID
	}{
		{ticketReview, &reservation}, {cashReview, nil},
	} {
		var link *uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT reservation_id FROM coach_earnings WHERE review_request_id=$1`, test.request).Scan(&link); err != nil {
			t.Fatal(err)
		}
		if test.want == nil && link != nil || test.want != nil && (link == nil || *link != *test.want) {
			t.Fatalf("earning for %s has reservation %v, want %v", test.request, link, test.want)
		}
	}
	exec(`INSERT INTO coaching_referral_coupons(owner_id,expires_at) VALUES ($1,now()+interval '6 months')`, owner)
	source, err := os.ReadFile("../../migrations/000187_referral_coupon.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, string(source)); err == nil || !strings.Contains(err.Error(), "cannot roll back migration 187") {
		t.Fatalf("down must stop when a coupon exists: %v", err)
	}
}

func coachingReferralCouponRestoredAfterReviewDeadlines(t *testing.T, h http.Handler) {
	admin := adminToken(t, h)
	_, owner, coupon, _ := grantReferralCoupon(t, h, admin)
	assertReleased := func(id string) {
		t.Helper()
		if rec := tick(t, h, "tick-secret"); rec.Code != 200 {
			t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
		}
		var released bool
		queryRowArgs(t, `SELECT EXISTS (SELECT FROM coaching_referral_coupons WHERE id=$1 AND used_at IS NULL AND used_reservation_id IS NULL)
            AND EXISTS (SELECT FROM coaching_ticket_reservations WHERE request_id=$2 AND status='released')`, []any{coupon, id}, &released)
		if !released {
			t.Fatalf("coupon was not restored for %s", id)
		}
	}
	queued := createCouponReview(t, h, owner)
	execSQLArgs(t, `UPDATE review_requests SET status='queued', coach_id=NULL, due_at=now()-interval '1 minute' WHERE id=$1`, queued)
	assertReleased(queued)

	onDutyCoach(t, h, admin)
	awaitingInfo := createCouponReview(t, h, owner)
	execSQLArgs(t, `UPDATE review_requests SET status='needs_info', info_from_status='in_review', info_request='確認したいこと',
        info_requested_at=now()-interval '25 hours', resubmit_by=now()-interval '1 minute' WHERE id=$1`, awaitingInfo)
	assertReleased(awaitingInfo)
}

func ticketReviewEarningRetainsReservationAfterWithdrawal(t *testing.T, h http.Handler) {
	admin := adminToken(t, h)
	ownerID, owner := demoRequester(t, h, admin)
	buySixTickets(t, h, owner)
	review := consumeOneTicketReview(t, h, admin, owner)
	execSQLArgs(t, `DELETE FROM profiles WHERE id=$1`, ownerID)
	var found bool
	queryRowArgs(t, `SELECT EXISTS (SELECT FROM coach_earnings e JOIN coaching_ticket_reservations r ON r.id=e.reservation_id
         WHERE e.review_request_id=$1 AND e.amount_yen=350 AND r.request_id IS NULL AND r.owner_id IS NULL)`, []any{review}, &found)
	if !found {
		t.Fatal("regular ticket earning lost its reservation link after withdrawal")
	}
}
