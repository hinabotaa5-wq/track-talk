package httpx_test

import (
	"encoding/json"
	"net/http"
	"strconv"
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
	queryRowArgs(t, `SELECT count(*) FROM coaching_ticket_ledger WHERE purchase_ref=$1`, []any{id}, &n)
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

// 本体 #691：コーチ詳細から来た購入は、その戻り先を購入記録に残し、確定後の照会でも返す。
func TestTicketPurchaseKeepsCoachDetailReturnPath(t *testing.T) {
	h, f := setupStripe(t)
	_, tu := demoUser(t, h, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
	coachPath := "/coaches/" + uuid.NewString()
	id := startTicketPurchase(t, h, tu, "six", uuid.NewString(), coachPath)
	var path string
	queryRowArgs(t, `SELECT return_path FROM coaching_ticket_purchases WHERE id=$1`, []any{id}, &path)
	if path != coachPath {
		t.Fatalf("return path %q", path)
	}
	// 購入画面を閉じた場合も戻り先を引き継ぐ（取消し時の挙動そのものは従来どおり購入画面に戻る）。
	if len(f.sessions) != 1 || !strings.Contains(f.sessions[0].Get("cancel_url"), "return_to="+strings.ReplaceAll(coachPath, "/", "%2F")) {
		t.Fatalf("checkout sessions %v", f.sessions)
	}
	payTicketPurchase(t, h, id, 2500)
	r := doJSON(t, h, http.MethodGet, "/v1/coaching/tickets/purchases/"+id, tu, nil)
	var out struct {
		Status     string `json:"status"`
		ReturnPath string `json:"return_path"`
	}
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &out) != nil || out.Status == "pending" || out.ReturnPath != coachPath {
		t.Fatalf("purchase %d %s", r.Code, r.Body.String())
	}
	for _, bad := range []string{coachPath + "/x", "/coaches/not-a-uuid", "https://evil.test" + coachPath} {
		bid := startTicketPurchase(t, h, tu, "six", uuid.NewString(), bad)
		queryRowArgs(t, `SELECT return_path FROM coaching_ticket_purchases WHERE id=$1`, []any{bid}, &path)
		if path != "/me/coaching/tickets" {
			t.Fatalf("unsafe return %q -> %q", bad, path)
		}
	}
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
	offers := coachRequests(t, h, tc).Reviewing
	if len(offers) != 1 {
		t.Fatalf("offers %+v", offers)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+offers[0].ID+"/start", tc, nil); rec.Code != 200 {
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
	offer := coachRequests(t, h, tc).Reviewing[0]
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+offer.ID+"/start", tc, nil); rec.Code != 200 {
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

// 失効後の返金（本体 #572 項目7）。運営者またはコーチの都合で有効期限内に受けられなかったことが
// 確認できた失効済みチケットは、申出ベースで1枚500円を元の決済へ返金し、1枚ずつ記録する。
func TestPostExpiryTicketRefundForOperatorFault(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	admin := adminToken(t, h)
	_, tu := demoRequester(t, h, admin)
	purchase := buySixTickets(t, h, tu)
	// 文面の版を購入ごとに残す（本体 #572 項目10）。
	var wording *string
	queryRowArgs(t, `SELECT wording_version FROM coaching_ticket_purchases WHERE id=$1`, []any{purchase}, &wording)
	if wording == nil || *wording != "v3" {
		t.Fatalf("wording version %+v", wording)
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/me/coaching/tickets", tu, nil); !strings.Contains(rec.Body.String(), `"terms_version":"v3"`) {
		t.Fatalf("terms version %s", rec.Body.String())
	}
	// 期限を過ぎたら、失効の tick を待たずに従来の返金では断る（既存の挙動を維持）。
	execSQLArgs(t, `UPDATE coaching_ticket_lots SET expires_at=now()-interval '1 minute' WHERE purchase_id=$1`, purchase)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/tickets/"+purchase+"/refund", admin, nil); rec.Code != 422 || !strings.Contains(rec.Body.String(), "tickets_expired") {
		t.Fatalf("legacy expired refund %d %s", rec.Code, rec.Body.String())
	}
	// 失効後の返金は1枚500円に相当する額を返し、台帳には1枚ずつ記録する。
	// 全6枚では3,000円で元の決済額（2,500円）を超えるので、Stripe へは2,500円、残りの500円は Stripe の外で返す（運営者決定 2026-11-12）。
	res := postExpiryRefund(t, h, admin, purchase, 6, 200)
	if res.AmountYen != 3000 || res.StripeYen != 2500 || res.ManualYen != 500 || res.Status != "succeeded" {
		t.Fatalf("post-expiry split %+v", res)
	}
	if len(f.refundList()) != 1 {
		t.Fatalf("remote refunds %d", len(f.refundList()))
	}
	amount, err := strconv.Atoi(f.refundList()[0].Get("amount"))
	if err != nil {
		t.Fatal(err)
	}
	if amount != 2500 {
		t.Fatalf("remote refund total %d", amount)
	}
	// 購入の返金済み額は Stripe 分＋Stripe 外の分（000162 の上限の判定に含める）。
	state := ticketSummary(t, h, tu)
	if state.Balance.Available != 0 || state.Purchases[0].RefundedYen != 3000 || state.Purchases[0].Status != "refunded" {
		t.Fatalf("post-expiry %+v", state)
	}
	var available, expired, refunded int
	queryRowArgs(t, `SELECT available,expired,refunded FROM coaching_ticket_lots WHERE purchase_id=$1`, []any{purchase}, &available, &expired, &refunded)
	if available != 0 || expired != 0 || refunded != 6 {
		t.Fatalf("lot %d %d %d", available, expired, refunded)
	}
	var expireRows, refundRows int
	queryRowArgs(t, `SELECT count(*) FILTER (WHERE kind='expire'),count(*) FILTER (WHERE kind='refund') FROM coaching_ticket_ledger WHERE purchase_ref=$1`, []any{purchase}, &expireRows, &refundRows)
	if expireRows != 1 || refundRows != 6 {
		t.Fatalf("ledger expire %d refund %d", expireRows, refundRows)
	}
	// 管理画面では Stripe 外の分が支払い待ちとして見える。
	attention := ticketAttentionRefunds(t, h, admin)
	if len(attention) != 1 || attention[0].ManualYen != 500 || attention[0].AmountYen != 3000 || attention[0].ManualCompletedAt != nil {
		t.Fatalf("attention %+v", attention)
	}
	// 運営者が振込などで返した後、支払方法と日付を記録して完了にする。
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/tickets/refunds/"+res.RefundID+"/manual-complete", tu, map[string]any{"method": "銀行振込", "paid_on": "2026-11-12"}); rec.Code != 403 {
		t.Fatalf("nonadmin manual %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/tickets/refunds/"+res.RefundID+"/manual-complete", admin, map[string]any{"method": "銀行振込", "paid_on": "12/11"}); rec.Code != 422 {
		t.Fatalf("bad date %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/tickets/refunds/"+res.RefundID+"/manual-complete", admin, map[string]any{"method": "銀行振込", "paid_on": "2026-11-12"}); rec.Code != 200 {
		t.Fatalf("manual complete %d %s", rec.Code, rec.Body.String())
	}
	var method, paidOn string
	queryRowArgs(t, `SELECT manual_method, manual_paid_on::text FROM coaching_ticket_refunds WHERE id=$1`, []any{res.RefundID}, &method, &paidOn)
	if method != "銀行振込" || paidOn != "2026-11-12" {
		t.Fatalf("manual record %s %s", method, paidOn)
	}
	// 二重に完了させない。返金済み額にも二重に足さない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/tickets/refunds/"+res.RefundID+"/manual-complete", admin, map[string]any{"method": "銀行振込", "paid_on": "2026-11-12"}); rec.Code != 422 {
		t.Fatalf("double manual %d %s", rec.Code, rec.Body.String())
	}
	if got := ticketSummary(t, h, tu).Purchases[0].RefundedYen; got != 3000 {
		t.Fatalf("refunded after manual %d", got)
	}
	// 返し切った分はもう返金できない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/tickets/"+purchase+"/refund", admin, map[string]any{"post_expiry": true, "quantity": 1}); rec.Code != 422 || !strings.Contains(rec.Body.String(), "invalid_refund_quantity") {
		t.Fatalf("over refund %d %s", rec.Code, rec.Body.String())
	}
	assertTicketAccounting(t, h, admin)
}

// サービス終了時の払戻しは、購入金額 ÷ 6 × 未使用枚数で、1円未満の端数は四捨五入する（本体 #572 v3）。
func TestTicketServiceEndRefundRoundsHalfUp(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	_, tc := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)
	purchase := buySixTickets(t, h, tu)
	for i := 0; i < 5; i++ {
		review := ticketReview(t, h, tu)
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
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/tickets/"+purchase+"/refund", admin, nil); rec.Code != 202 {
		t.Fatalf("refund %d %s", rec.Code, rec.Body.String())
	}
	// 2500 ÷ 6 × 1 = 416.66… → 四捨五入で417。
	if got := ticketSummary(t, h, tu).Purchases[0].RefundedYen; got != 417 {
		t.Fatalf("rounded refund %d", got)
	}
	// 有効期限の前は失効後の返金を使えない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/tickets/"+purchase+"/refund", admin, map[string]any{"post_expiry": true, "quantity": 1}); rec.Code != 422 || !strings.Contains(rec.Body.String(), "tickets_not_expired") {
		t.Fatalf("pre-expiry %d %s", rec.Code, rec.Body.String())
	}
	assertTicketAccounting(t, h, admin)
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

type postExpiryRefundResult struct {
	RefundID  string `json:"refund_id"`
	Status    string
	AmountYen int `json:"amount_yen"`
	StripeYen int `json:"stripe_yen"`
	ManualYen int `json:"manual_yen"`
}

func postExpiryRefund(t *testing.T, h http.Handler, admin, purchase string, quantity, want int) postExpiryRefundResult {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/tickets/"+purchase+"/refund", admin, map[string]any{"post_expiry": true, "quantity": quantity})
	if rec.Code != want {
		t.Fatalf("post-expiry refund %d %s", rec.Code, rec.Body.String())
	}
	var out postExpiryRefundResult
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

type ticketAttentionRefund struct {
	ID                string
	Status            string
	AmountYen         int        `json:"amount_yen"`
	ManualYen         int        `json:"manual_yen"`
	ManualCompletedAt *time.Time `json:"manual_completed_at"`
}

func ticketAttentionRefunds(t *testing.T, h http.Handler, admin string) []ticketAttentionRefund {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/admin/coaching/tickets", admin, nil)
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var out struct {
		Attention []ticketAttentionRefund `json:"attention_refunds"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.Attention
}

// 5枚（2,500円）までは元の決済額に収まるので全額を Stripe で返し、Stripe 外の分は0。
// 残りの1枚は Stripe の残りが0なので、Stripe へは送らずに全額を Stripe 外で返す（本体 #572）。
func TestPostExpiryTicketRefundWithinChargeIsAllStripe(t *testing.T) {
	h, f := setupStripe(t)
	admin := adminToken(t, h)
	_, tu := demoRequester(t, h, admin)
	purchase := buySixTickets(t, h, tu)
	execSQLArgs(t, `UPDATE coaching_ticket_lots SET expires_at=now()-interval '1 minute' WHERE purchase_id=$1`, purchase)
	res := postExpiryRefund(t, h, admin, purchase, 5, 200)
	if res.StripeYen != 2500 || res.ManualYen != 0 || len(f.refundList()) != 1 || f.refundList()[0].Get("amount") != "2500" {
		t.Fatalf("five tickets %+v %v", res, f.refundList())
	}
	if attention := ticketAttentionRefunds(t, h, admin); len(attention) != 0 {
		t.Fatalf("attention %+v", attention)
	}
	res = postExpiryRefund(t, h, admin, purchase, 1, 200)
	if res.StripeYen != 0 || res.ManualYen != 500 || len(f.refundList()) != 1 {
		t.Fatalf("sixth ticket %+v %v", res, f.refundList())
	}
	if got := ticketSummary(t, h, tu).Purchases[0].RefundedYen; got != 3000 {
		t.Fatalf("refunded %d", got)
	}
	assertTicketAccounting(t, h, admin)
}

// 先に別の経路（期限前の通常の返金）で Stripe へ返した分があれば、Stripe の残りはその分だけ減る（本体 #572）。
func TestPostExpiryTicketRefundAfterOtherRefundUsesRemainingCharge(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	admin := adminToken(t, h)
	onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)
	purchase := buySixTickets(t, h, tu)
	// 1枚を申込みで確保したまま、残りの5枚を期限前の通常の返金で返す（2500 ÷ 6 × 5 = 2083.33… → 2083円）。
	request := ticketReview(t, h, tu)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/tickets/"+purchase+"/refund", admin, nil); rec.Code != 202 {
		t.Fatalf("refund %d %s", rec.Code, rec.Body.String())
	}
	if got := ticketSummary(t, h, tu).Purchases[0].RefundedYen; got != 2083 {
		t.Fatalf("normal refund %d", got)
	}
	// 期限を過ぎてから申込みが引き受けられずに終わり、確保していた1枚は tick で失効に回る。
	execSQLArgs(t, `UPDATE coaching_ticket_lots SET expires_at=now()-interval '1 minute' WHERE purchase_id=$1`, purchase)
	execSQLArgs(t, `UPDATE review_requests SET status='expired' WHERE id=$1`, request)
	if rec := tick(t, h, "tick-secret"); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var expired int
	queryRowArgs(t, `SELECT expired FROM coaching_ticket_lots WHERE purchase_id=$1`, []any{purchase}, &expired)
	if expired != 1 {
		t.Fatalf("expired %d", expired)
	}
	// 失効後の返金500円のうち、Stripe へは残りの417円（2500 − 2083）、83円は Stripe 外で返す。
	res := postExpiryRefund(t, h, admin, purchase, 1, 200)
	if res.AmountYen != 500 || res.StripeYen != 417 || res.ManualYen != 83 {
		t.Fatalf("split after other refund %+v", res)
	}
	remote := f.refundList()
	if len(remote) != 2 || remote[1].Get("amount") != "417" {
		t.Fatalf("remote %v", remote)
	}
	if got := ticketSummary(t, h, tu).Purchases[0].RefundedYen; got != 2583 {
		t.Fatalf("refunded %d", got)
	}
	assertTicketAccounting(t, h, admin)
}

// Stripe が金額の超過で返金を断ったら（Stripe のダッシュボードで別に返金していた場合など）、
// 再試行を続けずに運営者の対応待ちにし、管理 API もそれと分かる応答を返す（本体 #572）。
func TestPostExpiryTicketRefundRejectedByStripeStopsRetrying(t *testing.T) {
	h, f := setupStripe(t)
	admin := adminToken(t, h)
	_, tu := demoRequester(t, h, admin)
	purchase := buySixTickets(t, h, tu)
	f.mu.Lock()
	f.chargeAmounts = map[string]int{"pi_ticket_" + purchase: 2000}
	f.mu.Unlock()
	execSQLArgs(t, `UPDATE coaching_ticket_lots SET expires_at=now()-interval '1 minute' WHERE purchase_id=$1`, purchase)
	res := postExpiryRefund(t, h, admin, purchase, 5, 409)
	if res.Status != "needs_action" {
		t.Fatalf("status %+v", res)
	}
	for i := 0; i < 2; i++ {
		if rec := tick(t, h, "tick-secret"); rec.Code != 200 {
			t.Fatal(rec.Body.String())
		}
	}
	var status string
	queryRowArgs(t, `SELECT status FROM coaching_ticket_refunds WHERE id=$1`, []any{res.RefundID}, &status)
	if status != "needs_action" || len(f.refundList()) != 0 {
		t.Fatalf("retried %s %v", status, f.refundList())
	}
	attention := ticketAttentionRefunds(t, h, admin)
	if len(attention) != 1 || attention[0].Status != "needs_action" {
		t.Fatalf("attention %+v", attention)
	}
	if got := ticketSummary(t, h, tu).Purchases[0].RefundedYen; got != 0 {
		t.Fatalf("refunded before manual %d", got)
	}
	// 運営者が Stripe の外で全額を返したら、完了として記録し、返金済み額に足す。
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/tickets/refunds/"+res.RefundID+"/manual-complete", admin, map[string]any{"method": "銀行振込", "paid_on": "2026-11-12"}); rec.Code != 200 {
		t.Fatalf("manual complete %d %s", rec.Code, rec.Body.String())
	}
	var manual int
	queryRowArgs(t, `SELECT status, manual_yen FROM coaching_ticket_refunds WHERE id=$1`, []any{res.RefundID}, &status, &manual)
	if status != "succeeded" || manual != 2500 {
		t.Fatalf("converted %s %d", status, manual)
	}
	if got := ticketSummary(t, h, tu).Purchases[0].RefundedYen; got != 2500 {
		t.Fatalf("refunded after manual %d", got)
	}
	assertTicketAccounting(t, h, admin)
}
