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
		rec := doJSON(t, h, "POST", "/v1/coaching/tickets/checkout", token, map[string]any{
			"product_id": "six", "request_key": uuid.NewString(), "guardian_permission": true, "referral_code": referral,
		})
		var data struct {
			ID string `json:"purchase_id"`
		}
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &data) != nil || data.ID == "" {
			t.Fatalf("checkout %d %s", rec.Code, rec.Body.String())
		}
		payTicketPurchase(t, h, data.ID, 2500)
		return data.ID
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
	payTicketPurchase(t, h, pack, 2500)
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
	var lot uuid.UUID
	var correctExpiry bool
	queryRowArgs(t, `SELECT l.id, l.expires_at=r.granted_at + interval '6 months'
		FROM coaching_referral_rewards r JOIN coaching_ticket_lots l ON l.id=r.lot_id
		WHERE r.referrer_id=$1 AND l.kind='referral_reward' AND l.purchase_id IS NULL AND l.available=1`,
		[]any{ownerID}, &lot, &correctExpiry)
	if !correctExpiry {
		t.Fatal("wrong reward expiry")
	}
	notices, _ := listNotifications(t, h, owner)
	count := 0
	for _, n := range notices {
		if strings.Contains(n.Title, "紹介特典") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("reward notifications: %d", count)
	}

	_, afterReward := demoRequester(t, h, admin)
	checkout(afterReward, code)
	assertCount(2)

	// This is the purchase status transition made by the completed refund.
	execSQLArgs(t, "UPDATE coaching_ticket_purchases SET refunded_yen=amount_yen, status='refunded' WHERE id=$1", pack)
	assertCount(1)
	var available int
	queryRowArgs(t, "SELECT available FROM coaching_ticket_lots WHERE id=$1", []any{lot}, &available)
	if available != 0 {
		t.Fatalf("canceled reward still has %d available", available)
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
