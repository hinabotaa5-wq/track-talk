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
	// 18歳未満のときに要る購入のたびの宣言も付ける（18歳以上には効かない）。
	r := doJSON(t, h, http.MethodPost, "/v1/coaching/tickets/checkout", tok, map[string]any{"product_id": product, "request_key": key, "return_path": path, "guardian_permission": true})
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
// buySixTickets は、6枚つづり（2,500円）を買って支払いまで済ませる（本体 #569）。
func buySixTickets(t *testing.T, h http.Handler, tok string) string {
	t.Helper()
	id := startTicketPurchase(t, h, tok, "six", uuid.NewString(), "/coaching/new")
	payTicketPurchase(t, h, id, 2500)
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
	id := startTicketPurchase(t, h, tu, "six", key, "https://evil.test")
	if got := startTicketPurchase(t, h, tu, "six", key, "https://evil.test"); got != id {
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
	payTicketPurchase(t, h, id, 2500)
	payTicketPurchase(t, h, id, 2500)
	if got := ticketSummary(t, h, tu).Balance.Available; got != 6 {
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
	buySixTickets(t, h, tu)
	id := ticketReview(t, h, tu)
	state := ticketSummary(t, h, tu)
	if state.Balance.Available != 5 || state.Balance.Reserved != 1 {
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
	if state.Balance.Available != 5 || state.Balance.Reserved != 0 {
		t.Fatalf("consume %+v", state.Balance)
	}
	var gross, earnings int
	queryRowArgs(t, `SELECT gross_yen,amount_yen FROM coach_earnings WHERE review_request_id=$1 AND coach_id=$2`, []any{id, coach}, &gross, &earnings)
	// まとめ買いの割引は運営が負担し、報酬は単発と同じ350円（500円の70%）に固定する。
	if gross != 500 || earnings != 350 {
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
	if state.Balance.Available != 5 || state.Balance.Reserved != 0 {
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
	buySixTickets(t, h, tu)
	bodies := make([]map[string]any, 7)
	for i := range bodies {
		bodies[i] = reviewBodyFor(t, h, tu)
		bodies[i]["method"] = "tickets"
	}
	start := make(chan struct{})
	codes := make([]int, len(bodies))
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
	if success != 6 || short != 1 || state.Balance.Available != 0 || state.Balance.Reserved != 6 {
		t.Fatalf("overspend %v %+v", codes, state.Balance)
	}
	var count int
	queryRowArgs(t, `SELECT count(*) FROM review_requests`, nil, &count)
	if count != 6 {
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
	purchase := buySixTickets(t, h, tu)
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
	if state.Balance.Available != 0 || state.Purchases[0].RefundedYen != 2083 || len(f.refunds) != 1 {
		t.Fatalf("refund %+v remote %v", state, f.refunds)
	}
	// 外部返金後にローカル確定だけ失敗した状態を再現する。操作IDで検索し、2回目の返金を作らない。
	execSQLArgs(t, `UPDATE coaching_ticket_refunds SET status='pending',stripe_refund_id=NULL WHERE purchase_id=$1`, purchase)
	execSQLArgs(t, `UPDATE coaching_ticket_purchases SET status='paid',refunded_yen=0 WHERE id=$1`, purchase)
	if rec := tick(t, h, "tick-secret"); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	state = ticketSummary(t, h, tu)
	if len(f.refunds) != 1 || state.Purchases[0].RefundedYen != 2083 {
		t.Fatalf("duplicate refund %+v remote %v", state, f.refunds)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/tickets/"+purchase+"/refund", admin, nil); rec.Code != 422 {
		t.Fatalf("refund consumed tickets %d %s", rec.Code, rec.Body.String())
	}
	assertTicketAccounting(t, h, admin)
}

// 18歳未満は保護者の同意なしにチケットを買えるが、単発と同じく購入のたびに宣言のチェックが要る（本体 #569）。
// 宣言は購入ごとに記録し、月の上限では止めない。買ったチケットは宣言なしで使える。
func TestTicketMinorPurchaseRequiresDeclaration(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	minor, tu := demoUser(t, h, time.Date(time.Now().Year()-14, 1, 1, 0, 0, 0, 0, time.UTC))
	body := map[string]any{"product_id": "six", "request_key": uuid.NewString(), "return_path": "/coaching/new"}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/tickets/checkout", tu, body); rec.Code != 422 || !strings.Contains(rec.Body.String(), "guardian_permission_required") {
		t.Fatalf("missing declaration %d %s", rec.Code, rec.Body.String())
	}
	if sessionCount(f) != 0 {
		t.Fatal("checkout created without declaration")
	}
	body["guardian_permission"] = true
	for i := 0; i < 2; i++ {
		body["request_key"] = uuid.NewString()
		rec := doJSON(t, h, http.MethodPost, "/v1/coaching/tickets/checkout", tu, body)
		if rec.Code != 200 {
			t.Fatalf("declared checkout %d %s", rec.Code, rec.Body.String())
		}
		var out struct {
			ID string `json:"purchase_id"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		payTicketPurchase(t, h, out.ID, 2500)
	}
	var declarations, amount int
	queryRowArgs(t, `SELECT count(*),COALESCE(max(amount_yen),0) FROM minor_purchase_declarations WHERE minor_id=$1 AND ref_type='ticket_purchase'`, []any{minor}, &declarations, &amount)
	if declarations != 2 || amount != 2500 {
		t.Fatalf("declarations %d amount %d", declarations, amount)
	}
	if got := ticketSummary(t, h, tu).Balance.Available; got != 12 {
		t.Fatalf("balance %d", got)
	}
	ticketReview(t, h, tu)
	assertTicketAccounting(t, h, adminToken(t, h))
}

func TestTicketRefundExcludesReservedAndCanRefundAfterRelease(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	admin := adminToken(t, h)
	_, tu := demoRequester(t, h, admin)
	purchase := buySixTickets(t, h, tu)
	review := ticketReview(t, h, tu)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/tickets/"+purchase+"/refund", admin, nil); rec.Code != 202 {
		t.Fatal(rec.Body.String())
	}
	state := ticketSummary(t, h, tu)
	if state.Balance.Reserved != 1 || state.Balance.Available != 0 || state.Purchases[0].RefundedYen != 2083 {
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
	if state.Balance.Available != 0 || state.Purchases[0].RefundedYen != 2500 || state.Purchases[0].Status != "refunded" || len(f.refunds) != 2 {
		t.Fatalf("full refund %+v", state)
	}
	assertTicketAccounting(t, h, admin)
}

func TestTicketLatePaymentAfterAccountDeletionIsRefunded(t *testing.T) {
	h, f := setupStripe(t)
	_, tu := demoUser(t, h, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
	purchase := startTicketPurchase(t, h, tu, "six", uuid.NewString(), "/coaching/new")
	// 受付中の決済がある間は退会を止める。
	if rec := doJSON(t, h, http.MethodDelete, "/v1/me", tu, map[string]any{"confirm": "退会する"}); rec.Code != 409 || !strings.Contains(rec.Body.String(), "tickets_remaining") {
		t.Fatalf("delete pending %d %s", rec.Code, rec.Body.String())
	}
	execSQLArgs(t, `UPDATE coaching_ticket_purchases SET session_expires_at=now()-interval '1 hour' WHERE id=$1`, purchase)
	if rec := doJSON(t, h, http.MethodDelete, "/v1/me", tu, map[string]any{"confirm": "退会する"}); rec.Code != 200 {
		t.Fatalf("delete %d %s", rec.Code, rec.Body.String())
	}
	payTicketPurchase(t, h, purchase, 2500)
	if rec := tick(t, h, "tick-secret"); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var status string
	queryRowArgs(t, `SELECT status FROM coaching_ticket_purchases WHERE id=$1`, []any{purchase}, &status)
	if status != "refunded" || len(f.refunds) != 1 {
		t.Fatalf("orphan payment %s %v", status, f.refunds)
	}
	payTicketPurchase(t, h, purchase, 2500)
	if len(f.refunds) != 1 {
		t.Fatal("duplicate orphan refund")
	}
	assertTicketAccounting(t, h, adminToken(t, h))
}

func TestTicketPendingRefundFailureRestoresBalance(t *testing.T) {
	h, f := setupStripe(t)
	admin := adminToken(t, h)
	_, tu := demoUser(t, h, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
	id := buySixTickets(t, h, tu)
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
	if state.Balance.Available != 6 || state.Purchases[0].RefundedYen != 0 {
		t.Fatalf("failed refund %+v", state)
	}
	f.mu.Lock()
	f.ticketRefundStatus = "succeeded"
	f.mu.Unlock()
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/tickets/"+id+"/refund", admin, nil); rec.Code != 202 {
		t.Fatal(rec.Body.String())
	}
	state = ticketSummary(t, h, tu)
	if state.Balance.Available != 0 || state.Purchases[0].RefundedYen != 2500 || len(f.refunds) != 2 {
		t.Fatalf("retry refund %+v", state)
	}
	assertTicketAccounting(t, h, admin)
}
