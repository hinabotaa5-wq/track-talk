package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestCoachingReferralFromPackAndCapturedReview(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	_, coach := onDutyCoach(t, h, admin)
	ownerID, owner := demoRequester(t, h, admin)
	_, buyer := demoRequester(t, h, admin)
	_, reviewer := demoRequester(t, h, admin)
	buySixTickets(t, h, owner)

	var code string
	queryRowArgs(t, "SELECT code FROM coaching_referral_codes WHERE owner_id=$1", []any{ownerID}, &code)
	if len(code) != 6 {
		t.Fatalf("unexpected code length: %q", code)
	}
	checkout := func(token, referral string) string {
		t.Helper()
		purchaseID := seedTicketPurchase(t, token, "six", uuid.NewString(), "/coaching/new", referral)
		payTicketPurchase(t, h, purchaseID, 5000)
		return purchaseID
	}
	assertCount := func(want int) {
		t.Helper()
		var count int
		queryRowArgs(t, "SELECT count(*) FROM coaching_referral_counts WHERE referrer_id=$1 AND canceled_at IS NULL", []any{ownerID}, &count)
		if count != want {
			t.Fatalf("friend count = %d, want %d", count, want)
		}
	}
	checkout(owner, code)
	checkout(buyer, "INVALID")
	assertCount(0)
	pack := checkout(buyer, "  "+strings.ToLower(code)+"  ")
	payTicketPurchase(t, h, pack, 5000)
	checkout(buyer, code)
	assertCount(1)

	body := reviewBodyFor(t, h, reviewer)
	body["method"], body["referral_code"] = "card", code
	rec := createReviewWith(t, h, reviewer, "card", body)
	var review reviewRequestView
	if rec.Code != 201 || json.Unmarshal(rec.Body.Bytes(), &review) != nil || review.ID == "" {
		t.Fatalf("review checkout %d %s", rec.Code, rec.Body.String())
	}
	var session string
	queryRowArgs(t, "SELECT checkout_session_id FROM review_payments WHERE request_id=$1", []any{review.ID}, &session)
	if rec := stripeWebhook(t, h, testWebhookSecret, "checkout.session.completed", map[string]any{
		"id": session, "payment_status": "unpaid", "payment_intent": "pi_card_referral",
		"metadata": map[string]string{"kind": "review"},
	}); rec.Code != 200 {
		t.Fatalf("authorize %d %s", rec.Code, rec.Body.String())
	}
	assertCount(1)
	if rec := doJSON(t, h, "POST", "/v1/coach/requests/"+review.ID+"/start", coach, nil); rec.Code != 200 {
		t.Fatalf("start %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, "POST", "/v1/coaching/requests/"+review.ID+"/response", coach, responseBody()); rec.Code != 200 {
		t.Fatalf("return %d %s", rec.Code, rec.Body.String())
	}
	assertCount(2)
	// 本体 #1031 G1・G2：周の達成で ¥500 クーポンが1枚付与され（付与から6か月有効・未使用）、特典チケットのロットは付与されない。
	var coupon uuid.UUID
	var correctExpiry bool
	queryRowArgs(t, `SELECT c.id, c.expires_at=r.granted_at + interval '6 months' AND c.status='unused' AND c.amount_yen=500 AND c.source='referral' AND c.owner_id=$1
		FROM coaching_referral_rewards r JOIN coaching_coupons c ON c.id=r.coupon_id
		WHERE r.referrer_id=$1 AND r.lot_id IS NULL`,
		[]any{ownerID}, &coupon, &correctExpiry)
	if !correctExpiry {
		t.Fatal("wrong reward coupon")
	}
	var rewardLots int
	queryRowArgs(t, `SELECT count(*) FROM coaching_ticket_lots WHERE owner_id=$1 AND kind='referral_reward'`, []any{ownerID}, &rewardLots)
	if rewardLots != 0 {
		t.Fatalf("reward ticket lots: %d", rewardLots)
	}
	// G3：付与の通知はクーポンの文面で届く（「無料チケット」の文言を含まない）。
	notices := coachingNotifications(t, h, owner)
	count := 0
	for _, n := range notices {
		if strings.Contains(n.Title, "紹介特典") {
			count++
			if n.Title != "紹介特典：¥500クーポンをプレゼントしました" || strings.Contains(n.Title+n.Body, "無料チケット") || !strings.Contains(n.Body, "6か月有効") {
				t.Fatalf("reward notification %+v", n)
			}
		}
	}
	if count != 1 {
		t.Fatalf("reward notifications: %d", count)
	}
	// G3：紹介進捗 API は特典の種類と有効期限を返し、「無料チケット」の文言を含まない。
	if rec := doJSON(t, h, http.MethodGet, "/v1/me/coaching/referral", owner, nil); rec.Code != 200 ||
		!strings.Contains(rec.Body.String(), `"reward_kind":"coupon"`) || strings.Contains(rec.Body.String(), "無料チケット") {
		t.Fatalf("referral progress %d %s", rec.Code, rec.Body.String())
	}

	_, afterReward := demoRequester(t, h, admin)
	checkout(afterReward, code)
	assertCount(2)

	// This is the purchase status transition made by the completed refund.
	execSQLArgs(t, "UPDATE coaching_ticket_purchases SET refunded_yen=amount_yen, status='refunded' WHERE id=$1", pack)
	assertCount(1)
	var couponStatus string
	queryRowArgs(t, "SELECT status FROM coaching_coupons WHERE id=$1", []any{coupon}, &couponStatus)
	if couponStatus != "canceled" {
		t.Fatalf("canceled reward coupon is %s", couponStatus)
	}
	checkout(afterReward, code)
	assertCount(2)
	var active int
	queryRowArgs(t, "SELECT count(*) FROM coaching_referral_rewards WHERE referrer_id=$1 AND canceled_at IS NULL", []any{ownerID}, &active)
	if active != 1 {
		t.Fatalf("active rewards = %d", active)
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/admin/coaching/referrals?code="+code, buyer, nil); rec.Code != 403 {
		t.Fatalf("referral lookup by non-admin %d", rec.Code)
	}
}
