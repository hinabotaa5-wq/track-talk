package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// 動画添削の再設計（本体 #569 フェーズA）の、お金の経路のテスト。

// チケットは決済完了から6か月で失効する。期限を過ぎた未使用分は tick で失効に回し（行は消さない）、
// 返却待ちで確保していた分は、返却されずに終わった時点で使える枚数に戻さず失効に回す。失効した分は返金しない。
func TestTicketsExpireSixMonthsAfterPayment(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	admin := adminToken(t, h)
	_, tu := demoRequester(t, h, admin)
	purchase := buySixTickets(t, h, tu)
	var months int
	queryRowArgs(t, `SELECT (l.expires_at = p.paid_at + interval '6 months')::int FROM coaching_ticket_lots l JOIN coaching_ticket_purchases p ON p.id=l.purchase_id WHERE l.purchase_id=$1`, []any{purchase}, &months)
	if months != 1 {
		t.Fatal("expiry is not six months after payment")
	}
	rec := doJSON(t, h, http.MethodGet, "/v1/me/coaching/tickets", tu, nil)
	if !strings.Contains(rec.Body.String(), `"next_expiry":{"available":6,"expires_at":`) {
		t.Fatalf("next expiry %s", rec.Body.String())
	}
	review := ticketReview(t, h, tu)

	execSQLArgs(t, `UPDATE coaching_ticket_lots SET expires_at=now()-interval '1 minute' WHERE purchase_id=$1`, purchase)
	// 失効の処理の前でも、期限を過ぎた分は使えない。
	if got := ticketSummary(t, h, tu).Balance.Available; got != 0 {
		t.Fatalf("expired tickets still usable %d", got)
	}
	body := reviewBodyFor(t, h, tu)
	body["method"] = "tickets"
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, body); rec.Code != 422 || !strings.Contains(rec.Body.String(), "insufficient_tickets") {
		t.Fatalf("expired use %d %s", rec.Code, rec.Body.String())
	}
	if rec := tick(t, h, "tick-secret"); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"tickets_expired":5`) {
		t.Fatalf("expire %d %s", rec.Code, rec.Body.String())
	}
	if rec := tick(t, h, "tick-secret"); !strings.Contains(rec.Body.String(), `"tickets_expired":0`) {
		t.Fatalf("expire again %s", rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/tickets/"+purchase+"/refund", admin, nil); rec.Code != 422 || !strings.Contains(rec.Body.String(), "tickets_expired") {
		t.Fatalf("expired refund %d %s", rec.Code, rec.Body.String())
	}
	// 確保中の1枚は、返却されずに終わったら失効に回す。
	execSQLArgs(t, `UPDATE review_requests SET due_at=now()-interval '1 minute' WHERE id=$1`, review)
	if rec := tick(t, h, "tick-secret"); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var available, reserved, expired int
	queryRowArgs(t, `SELECT available,reserved,expired FROM coaching_ticket_lots WHERE purchase_id=$1`, []any{purchase}, &available, &reserved, &expired)
	if available != 0 || reserved != 0 || expired != 6 {
		t.Fatalf("lot %d %d %d", available, reserved, expired)
	}
	var expireRows, expireTotal int
	queryRowArgs(t, `SELECT count(*),sum(expired_delta)::int FROM coaching_ticket_ledger WHERE purchase_ref=$1 AND kind='expire'`, []any{purchase}, &expireRows, &expireTotal)
	if expireRows != 2 || expireTotal != 6 {
		t.Fatalf("expire ledger %d %d", expireRows, expireTotal)
	}
	if len(f.refundList()) != 0 {
		t.Fatal("expired tickets refunded")
	}
	assertTicketAccounting(t, h, admin)
}

// 期限前に返却された添削は、期限を過ぎてもそのまま消費・報酬計上する（失効は未使用分だけ）。
func TestTicketReservedBeforeExpiryIsConsumedOnReturn(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)
	purchase := buySixTickets(t, h, tu)
	review := ticketReview(t, h, tu)
	execSQLArgs(t, `UPDATE coaching_ticket_lots SET expires_at=now()-interval '1 minute' WHERE purchase_id=$1`, purchase)
	offers := coachRequests(t, h, tc).Reviewing
	if len(offers) != 1 {
		t.Fatalf("offers %+v", offers)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+offers[0].ID+"/start", tc, nil); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+review+"/response", tc, responseBody()); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var consumed int
	queryRowArgs(t, `SELECT consumed FROM coaching_ticket_lots WHERE purchase_id=$1`, []any{purchase}, &consumed)
	var earnings int
	queryRowArgs(t, `SELECT amount_yen FROM coach_earnings WHERE review_request_id=$1 AND coach_id=$2`, []any{review, coach}, &earnings)
	if consumed != 1 || earnings != 350 {
		t.Fatalf("consumed %d earnings %d", consumed, earnings)
	}
	assertTicketAccounting(t, h, admin)
}

// 支払い画面から戻ったとき、URL ではなく Stripe の支払い画面の実際の状態で依頼を進める。
// Webhook と同じ処理を通すので、あとから Webhook が届いても二重にはならない。
func TestReviewPaymentSyncUsesStripeState(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	admin := adminToken(t, h)
	_, tc := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)
	body := reviewBodyFor(t, h, tu)
	body["method"] = "card"
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct{ ID string }
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	var session string
	queryRowArgs(t, `SELECT checkout_session_id FROM review_payments WHERE request_id=$1`, []any{created.ID}, &session)

	sync := func() map[string]any {
		t.Helper()
		rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+created.ID+"/payment/sync", tu, nil)
		if rec.Code != 200 {
			t.Fatalf("sync %d %s", rec.Code, rec.Body.String())
		}
		var out map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return out
	}
	// まだ払っていない（戻るボタン・キャンセル）なら、支払い待ちのまま。
	if out := sync(); out["status"] != "awaiting_payment" || out["checkout_status"] != "open" {
		t.Fatalf("open %+v", out)
	}
	if offers := coachRequests(t, h, tc).Reviewing; len(offers) != 0 {
		t.Fatal("offered before payment")
	}
	// ほかの人は確かめられない。
	_, other := demoRequester(t, h, admin)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+created.ID+"/payment/sync", other, nil); rec.Code != 404 {
		t.Fatalf("other %d", rec.Code)
	}
	f.setCheckoutState(session, map[string]any{"status": "complete", "payment_status": "unpaid", "payment_intent": "pi_card_sync", "metadata": map[string]string{"kind": "review"}})
	out := sync()
	if out["status"] == "awaiting_payment" || out["checkout_status"] != "complete" {
		t.Fatalf("complete %+v", out)
	}
	if reviewPaymentStatus(t, created.ID) != "authorized" {
		t.Fatal("not authorized")
	}
	// 後から届いた Webhook と、もう一度の確認では何も増えない。
	if rec := stripeWebhook(t, h, testWebhookSecret, "checkout.session.completed", map[string]any{"id": session, "payment_status": "unpaid", "payment_intent": "pi_card_sync", "metadata": map[string]any{"kind": "review"}}); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	sync()
	var payments, offers int
	queryRowArgs(t, `SELECT (SELECT count(*) FROM review_payments WHERE request_id=$1), (SELECT count(*) FROM review_offers WHERE request_id=$1)`, []any{created.ID}, &payments, &offers)
	if payments != 1 || offers != 0 {
		t.Fatalf("payments %d offers %d", payments, offers)
	}
}

// チケットの購入画面から戻ったときも、Stripe の実際の状態で残高を付ける。払っていなければ付けない。
func TestTicketPurchaseSyncUsesStripeState(t *testing.T) {
	h, f := setupStripe(t)
	_, tu := demoUser(t, h, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
	id := startTicketPurchase(t, h, tu, "six", uuid.NewString(), "/coaching/new")
	var session string
	queryRowArgs(t, `SELECT checkout_session_id FROM coaching_ticket_purchases WHERE id=$1`, []any{id}, &session)
	read := func() string {
		t.Helper()
		rec := doJSON(t, h, http.MethodGet, "/v1/coaching/tickets/purchases/"+id, tu, nil)
		if rec.Code != 200 {
			t.Fatalf("purchase %d %s", rec.Code, rec.Body.String())
		}
		return rec.Body.String()
	}
	if body := read(); !strings.Contains(body, `"status":"pending"`) || !strings.Contains(body, `"checkout_status":"open"`) || ticketSummary(t, h, tu).Balance.Available != 0 {
		t.Fatalf("open %s", body)
	}
	f.setCheckoutState(session, map[string]any{"status": "complete", "payment_status": "paid", "payment_intent": "pi_ticket_sync", "client_reference_id": id,
		"amount_total": 2500, "currency": "jpy", "metadata": map[string]string{"kind": "tickets", "purchase_id": id}})
	if body := read(); !strings.Contains(body, `"status":"paid"`) || strings.Contains(body, `"expires_at":null`) {
		t.Fatalf("paid %s", body)
	}
	payTicketPurchase(t, h, id, 2500)
	read()
	if got := ticketSummary(t, h, tu).Balance.Available; got != 6 {
		t.Fatalf("balance %d", got)
	}
	assertTicketAccounting(t, h, adminToken(t, h))
}

// 1つの申込みに支払い画面は1つ。同時に開き直しても、新しい画面は作らず同じ画面を返す。
func TestReviewCheckoutReopenIsSingleSession(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	_, tu := demoRequester(t, h, adminToken(t, h))
	body := reviewBodyFor(t, h, tu)
	body["method"] = "card"
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID          string
		CheckoutURL string `json:"checkout_url"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	before := sessionCount(f)
	start := make(chan struct{})
	urls := make([]string, 4)
	var wg sync.WaitGroup
	for i := range urls {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+created.ID+"/checkout", tu, map[string]any{"method": "card", "guardian_permission": true})
			var out struct {
				CheckoutURL string `json:"checkout_url"`
			}
			_ = json.Unmarshal(rec.Body.Bytes(), &out)
			urls[i] = out.CheckoutURL
		}(i)
	}
	close(start)
	wg.Wait()
	for _, u := range urls {
		if u != created.CheckoutURL {
			t.Fatalf("urls %v created %s", urls, created.CheckoutURL)
		}
	}
	var payments int
	queryRowArgs(t, `SELECT count(*) FROM review_payments WHERE request_id=$1`, []any{created.ID}, &payments)
	if sessionCount(f) != before || payments != 1 {
		t.Fatalf("sessions %d→%d payments %d", before, sessionCount(f), payments)
	}
}
