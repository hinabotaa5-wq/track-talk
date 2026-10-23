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

type ticketSummaryView struct {
	Balance   struct{ Available, Reserved int }
	Purchases []struct {
		ID, Status  string
		AmountYen   int `json:"amount_yen"`
		RefundedYen int `json:"refunded_yen"`
	}
}

func ticketSummary(t *testing.T, h http.Handler, tok string) ticketSummaryView {
	t.Helper()
	r := doJSON(t, h, http.MethodGet, "/v1/me/coaching/tickets", tok, nil)
	if r.Code != 200 {
		t.Fatalf("tickets %d %s", r.Code, r.Body.String())
	}
	var out ticketSummaryView
	if err := json.Unmarshal(r.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func startTicketPurchase(t *testing.T, h http.Handler, tok, product, key, path string) string {
	t.Helper()
	r := doJSON(t, h, http.MethodPost, "/v1/coaching/tickets/checkout", tok, map[string]any{"product_id": product, "request_key": key, "return_path": path})
	if r.Code != 200 {
		t.Fatalf("checkout %d %s", r.Code, r.Body.String())
	}
	var out struct {
		ID string `json:"purchase_id"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.ID
}
func payTicketPurchase(t *testing.T, h http.Handler, id string, amount int) {
	t.Helper()
	var session string
	queryRowArgs(t, `SELECT checkout_session_id FROM coaching_ticket_purchases WHERE id=$1`, []any{id}, &session)
	rec := stripeWebhook(t, h, testWebhookSecret, "checkout.session.completed", map[string]any{"id": session, "payment_status": "paid", "payment_intent": "pi_ticket_" + id,
		"client_reference_id": id, "amount_total": amount, "currency": "jpy", "metadata": map[string]string{"kind": "tickets", "purchase_id": id}})
	if rec.Code != 200 {
		t.Fatalf("pay tickets %d %s", rec.Code, rec.Body.String())
	}
}
func buyFiveTickets(t *testing.T, h http.Handler, tok string) string {
	t.Helper()
	id := startTicketPurchase(t, h, tok, "five", uuid.NewString(), "/coaching/new")
	payTicketPurchase(t, h, id, 2400)
	return id
}
func ticketReview(t *testing.T, h http.Handler, tok string) string {
	t.Helper()
	body := reviewBodyFor(t, h, tok)
	body["method"] = "tickets"
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tok, body)
	if rec.Code != 201 {
		t.Fatalf("ticket review %d %s", rec.Code, rec.Body.String())
	}
	var out struct{ ID string }
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.ID
}
func assertTicketAccounting(t *testing.T, h http.Handler, admin string) {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/admin/coaching/tickets", admin, nil)
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var out struct {
		Issues []string `json:"balance_issues"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Issues) != 0 {
		t.Fatalf("ledger mismatch: %v", out.Issues)
	}
}

func TestTicketPurchaseOnlyCreditsVerifiedWebhookOnce(t *testing.T) {
	h, f := setupStripe(t)
	_, tu := demoUser(t, h, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
	key := uuid.NewString()
	id := startTicketPurchase(t, h, tu, "five", key, "https://evil.test")
	if got := startTicketPurchase(t, h, tu, "five", key, "https://evil.test"); got != id {
		t.Fatalf("duplicate purchase %s %s", id, got)
	}
	if len(f.sessions) != 1 {
		t.Fatalf("duplicate checkout %d", len(f.sessions))
	}
	if got := ticketSummary(t, h, tu).Balance.Available; got != 0 {
		t.Fatalf("credited before payment: %d", got)
	}
	var session, path string
	queryRowArgs(t, `SELECT checkout_session_id,return_path FROM coaching_ticket_purchases WHERE id=$1`, []any{id}, &session, &path)
	if path != "/me/coaching/tickets" {
		t.Fatalf("unsafe return %q", path)
	}
	rec := stripeWebhook(t, h, testWebhookSecret, "checkout.session.completed", map[string]any{"id": session, "payment_status": "paid", "payment_intent": "pi_ticket_" + id, "client_reference_id": id, "amount_total": 1, "currency": "jpy", "metadata": map[string]string{"kind": "tickets", "purchase_id": id}})
	if rec.Code != 500 || ticketSummary(t, h, tu).Balance.Available != 0 {
		t.Fatalf("incorrect amount credited: %d", rec.Code)
	}
	payTicketPurchase(t, h, id, 2400)
	payTicketPurchase(t, h, id, 2400)
	if got := ticketSummary(t, h, tu).Balance.Available; got != 5 {
		t.Fatalf("credit %d", got)
	}
	var n int
	queryRowArgs(t, `SELECT count(*) FROM coaching_ticket_ledger WHERE lot_id=$1`, []any{id}, &n)
	if n != 1 {
		t.Fatalf("duplicate credit ledger %d", n)
	}
	_, other := demoUser(t, h, time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC))
	if rec := doJSON(t, h, http.MethodGet, "/v1/coaching/tickets/purchases/"+id, other, nil); rec.Code != 404 {
		t.Fatalf("purchase privacy %d", rec.Code)
	}
	if ticketSummary(t, h, other).Balance.Available != 0 {
		t.Fatal("other user balance")
	}
	assertTicketAccounting(t, h, adminToken(t, h))
}

func TestTicketsReservedConsumedAndReleasedWithFixedEarnings(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)
	buyFiveTickets(t, h, tu)
	id := ticketReview(t, h, tu)
	state := ticketSummary(t, h, tu)
	if state.Balance.Available != 3 || state.Balance.Reserved != 2 {
		t.Fatalf("reserve %+v", state.Balance)
	}
	offers := coachRequests(t, h, tc).Offers
	if len(offers) != 1 {
		t.Fatalf("offers %+v", offers)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/offers/"+offers[0].ID+"/accept", tc, nil); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", tc, responseBody()); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	state = ticketSummary(t, h, tu)
	if state.Balance.Available != 3 || state.Balance.Reserved != 0 {
		t.Fatalf("consume %+v", state.Balance)
	}
	var gross, earnings int
	queryRowArgs(t, `SELECT gross_yen,amount_yen FROM coach_earnings WHERE review_request_id=$1 AND coach_id=$2`, []any{id, coach}, &gross, &earnings)
	if gross != 1000 || earnings != 700 {
		t.Fatalf("discount affected earnings %d %d", gross, earnings)
	}
	if len(f.captured) != 0 || len(f.refunds) != 0 {
		t.Fatal("ticket usage charged or refunded cash")
	}
	id2 := ticketReview(t, h, tu)
	execSQLArgs(t, `UPDATE review_requests SET due_at=now()-interval '1 minute' WHERE id=$1`, id2)
	if rec := tick(t, h, "tick-secret"); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"tickets_released":1`) {
		t.Fatalf("release %d %s", rec.Code, rec.Body.String())
	}
	if rec := tick(t, h, "tick-secret"); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"tickets_released":0`) {
		t.Fatalf("repeat %s", rec.Body.String())
	}
	state = ticketSummary(t, h, tu)
	if state.Balance.Available != 3 || state.Balance.Reserved != 0 {
		t.Fatalf("released %+v", state.Balance)
	}
	rec := doJSON(t, h, http.MethodGet, "/v1/coaching/requests/"+id2, tu, nil)
	if !strings.Contains(rec.Body.String(), `"status":"released"`) {
		t.Fatal(rec.Body.String())
	}
	assertTicketAccounting(t, h, admin)
}

func TestConcurrentTicketReservationsCannotOverspend(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	_, tu := demoUser(t, h, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
	buyFiveTickets(t, h, tu)
	bodies := make([]map[string]any, 3)
	for i := range bodies {
		bodies[i] = reviewBodyFor(t, h, tu)
		bodies[i]["method"] = "tickets"
	}
	start := make(chan struct{})
	codes := make([]int, 3)
	var wg sync.WaitGroup
	for i := range bodies {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			codes[i] = doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, bodies[i]).Code
		}(i)
	}
	close(start)
	wg.Wait()
	success, short := 0, 0
	for _, c := range codes {
		if c == 201 {
			success++
		} else if c == 422 {
			short++
		} else {
			t.Fatalf("unexpected statuses %v", codes)
		}
	}
	state := ticketSummary(t, h, tu)
	if success != 2 || short != 1 || state.Balance.Available != 1 || state.Balance.Reserved != 4 {
		t.Fatalf("overspend %v %+v", codes, state.Balance)
	}
	var count int
	queryRowArgs(t, `SELECT count(*) FROM review_requests`, nil, &count)
	if count != 2 {
		t.Fatalf("failed reservation left request: %d", count)
	}
	assertTicketAccounting(t, h, adminToken(t, h))
}

func TestTicketUnusedRefundAndCrashRecovery(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	admin := adminToken(t, h)
	_, tc := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)
	purchase := buyFiveTickets(t, h, tu)
	request := ticketReview(t, h, tu)
	offer := coachRequests(t, h, tc).Offers[0]
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/offers/"+offer.ID+"/accept", tc, nil); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+request+"/response", tc, responseBody()); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/tickets/"+purchase+"/refund", tu, nil); rec.Code != 403 {
		t.Fatalf("nonadmin refund %d", rec.Code)
	}
	rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/tickets/"+purchase+"/refund", admin, nil)
	if rec.Code != 202 {
		t.Fatalf("refund %d %s", rec.Code, rec.Body.String())
	}
	state := ticketSummary(t, h, tu)
	if state.Balance.Available != 0 || state.Purchases[0].RefundedYen != 1440 || len(f.refunds) != 1 {
		t.Fatalf("refund %+v remote %v", state, f.refunds)
	}
	// 外部返金後にローカル確定だけ失敗した状態を再現する。操作IDで検索し、2回目の返金を作らない。
	execSQLArgs(t, `UPDATE coaching_ticket_refunds SET status='pending',stripe_refund_id=NULL WHERE purchase_id=$1`, purchase)
	execSQLArgs(t, `UPDATE coaching_ticket_purchases SET status='paid',refunded_yen=0 WHERE id=$1`, purchase)
	if rec := tick(t, h, "tick-secret"); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	state = ticketSummary(t, h, tu)
	if len(f.refunds) != 1 || state.Purchases[0].RefundedYen != 1440 {
		t.Fatalf("duplicate refund %+v remote %v", state, f.refunds)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/tickets/"+purchase+"/refund", admin, nil); rec.Code != 422 {
		t.Fatalf("refund consumed tickets %d %s", rec.Code, rec.Body.String())
	}
	assertTicketAccounting(t, h, admin)
}

func TestTicketGuardianConsentLimitsAndNoDoubleChargeOnUse(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	minor, tu := demoUser(t, h, time.Date(time.Now().Year()-14, 1, 1, 0, 0, 0, 0, time.UTC))
	body := map[string]any{"product_id": "five", "request_key": uuid.NewString(), "return_path": "/coaching/new"}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/tickets/checkout", tu, body); rec.Code != 422 || !strings.Contains(rec.Body.String(), "guardian_required") {
		t.Fatalf("missing consent %d %s", rec.Code, rec.Body.String())
	}
	gt := issueGuardianLink(t, h, tu, guardianEmail())
	if rec := doJSON(t, h, http.MethodPost, "/v1/guardian/"+gt+"/consent", "", map[string]any{"guardian_name": "保護者", "relationship": "mother", "agree": true, "monthly_limit_yen": 3000}); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	buyFiveTickets(t, h, tu)
	// 2400円購入済み。追加の2400円購入は不可、500円は可。
	body["request_key"] = uuid.NewString()
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/tickets/checkout", tu, body); rec.Code != 422 || !strings.Contains(rec.Body.String(), "monthly_limit") {
		t.Fatalf("monthly cap %d %s", rec.Code, rec.Body.String())
	}
	single := startTicketPurchase(t, h, tu, "single", uuid.NewString(), "/coaching/new")
	payTicketPurchase(t, h, single, 500)
	ticketReview(t, h, tu) // 2900円購入済みでも使用時に1000円を二重計上しない。
	execSQLArgs(t, `UPDATE guardian_consents SET status='revoked' WHERE minor_id=$1`, minor)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, func() map[string]any { b := reviewBodyFor(t, h, tu); b["method"] = "tickets"; return b }()); rec.Code != 422 {
		t.Fatalf("revoked use %d", rec.Code)
	}
	assertTicketAccounting(t, h, adminToken(t, h))
}

func TestTicketRefundExcludesReservedAndCanRefundAfterRelease(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	admin := adminToken(t, h)
	_, tu := demoRequester(t, h, admin)
	purchase := buyFiveTickets(t, h, tu)
	review := ticketReview(t, h, tu)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/tickets/"+purchase+"/refund", admin, nil); rec.Code != 202 {
		t.Fatal(rec.Body.String())
	}
	state := ticketSummary(t, h, tu)
	if state.Balance.Reserved != 2 || state.Balance.Available != 0 || state.Purchases[0].RefundedYen != 1440 {
		t.Fatalf("reserved refund %+v", state)
	}
	execSQLArgs(t, `UPDATE review_requests SET due_at=now()-interval '1 minute' WHERE id=$1`, review)
	if rec := tick(t, h, "tick-secret"); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/tickets/"+purchase+"/refund", admin, nil); rec.Code != 202 {
		t.Fatal(rec.Body.String())
	}
	state = ticketSummary(t, h, tu)
	if state.Balance.Available != 0 || state.Purchases[0].RefundedYen != 2400 || state.Purchases[0].Status != "refunded" || len(f.refunds) != 2 {
		t.Fatalf("full refund %+v", state)
	}
	assertTicketAccounting(t, h, admin)
}

func TestTicketLatePaymentAfterAccountDeletionIsRefunded(t *testing.T) {
	h, f := setupStripe(t)
	_, tu := demoUser(t, h, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
	purchase := startTicketPurchase(t, h, tu, "single", uuid.NewString(), "/coaching/new")
	// 受付中の決済がある間は退会を止める。
	if rec := doJSON(t, h, http.MethodDelete, "/v1/me", tu, map[string]any{"confirm": "退会する"}); rec.Code != 409 || !strings.Contains(rec.Body.String(), "tickets_remaining") {
		t.Fatalf("delete pending %d %s", rec.Code, rec.Body.String())
	}
	execSQLArgs(t, `UPDATE coaching_ticket_purchases SET session_expires_at=now()-interval '1 hour' WHERE id=$1`, purchase)
	if rec := doJSON(t, h, http.MethodDelete, "/v1/me", tu, map[string]any{"confirm": "退会する"}); rec.Code != 200 {
		t.Fatalf("delete %d %s", rec.Code, rec.Body.String())
	}
	payTicketPurchase(t, h, purchase, 500)
	if rec := tick(t, h, "tick-secret"); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var status string
	queryRowArgs(t, `SELECT status FROM coaching_ticket_purchases WHERE id=$1`, []any{purchase}, &status)
	if status != "refunded" || len(f.refunds) != 1 {
		t.Fatalf("orphan payment %s %v", status, f.refunds)
	}
	payTicketPurchase(t, h, purchase, 500)
	if len(f.refunds) != 1 {
		t.Fatal("duplicate orphan refund")
	}
	assertTicketAccounting(t, h, adminToken(t, h))
}

func TestTicketPendingRefundFailureRestoresBalance(t *testing.T) {
	h, f := setupStripe(t)
	admin := adminToken(t, h)
	_, tu := demoUser(t, h, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
	id := buyFiveTickets(t, h, tu)
	f.mu.Lock()
	f.ticketRefundStatus = "pending"
	f.mu.Unlock()
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/tickets/"+id+"/refund", admin, nil); rec.Code != 202 {
		t.Fatal(rec.Body.String())
	}
	state := ticketSummary(t, h, tu)
	if state.Balance.Available != 0 || state.Purchases[0].RefundedYen != 0 {
		t.Fatalf("pending refund %+v", state)
	}
	f.mu.Lock()
	f.ticketRefundStatus = "failed"
	f.mu.Unlock()
	if rec := tick(t, h, "tick-secret"); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	state = ticketSummary(t, h, tu)
	if state.Balance.Available != 5 || state.Purchases[0].RefundedYen != 0 {
		t.Fatalf("failed refund %+v", state)
	}
	f.mu.Lock()
	f.ticketRefundStatus = "succeeded"
	f.mu.Unlock()
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/tickets/"+id+"/refund", admin, nil); rec.Code != 202 {
		t.Fatal(rec.Body.String())
	}
	state = ticketSummary(t, h, tu)
	if state.Balance.Available != 0 || state.Purchases[0].RefundedYen != 2400 || len(f.refunds) != 2 {
		t.Fatalf("retry refund %+v", state)
	}
	assertTicketAccounting(t, h, admin)
}
