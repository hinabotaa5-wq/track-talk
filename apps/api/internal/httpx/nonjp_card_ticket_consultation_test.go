package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// カード発行国の確認をチケット購入と追加質問・追加相談の都度払いにも広げた分の受け入れテスト。
// 単発（仮押さえの取消し）は review_payment_nonjp_test.go。即時確定の2経路は、成果物を残さず全額返金にする。
// 偽 Stripe は pi_nonjp で始まる支払いを米国発行、pi_unknown で始まる支払いを発行国不明として答える。

// payTicketPurchaseWith は、指定した PaymentIntent でチケット購入の完了を知らせる。
func payTicketPurchaseWith(t *testing.T, h http.Handler, id, intent string, amount int) *httptest.ResponseRecorder {
	t.Helper()
	var session string
	queryRowArgs(t, `SELECT checkout_session_id FROM coaching_ticket_purchases WHERE id=$1`, []any{id}, &session)
	return stripeWebhook(t, h, testWebhookSecret, "checkout.session.completed", map[string]any{"id": session, "payment_status": "paid", "payment_intent": intent,
		"client_reference_id": id, "amount_total": amount, "currency": "jpy", "metadata": map[string]string{"kind": "tickets", "purchase_id": id}})
}

// payConsultationWith は、指定した PaymentIntent で追加質問・追加相談の支払い完了を知らせる。
func payConsultationWith(t *testing.T, h http.Handler, id, intent string, amount int) int {
	t.Helper()
	var session string
	queryRowArgs(t, `SELECT checkout_session_id FROM coaching_consultations WHERE id=$1`, []any{id}, &session)
	r := stripeWebhook(t, h, testWebhookSecret, "checkout.session.completed", map[string]any{"id": session, "payment_status": "paid", "payment_intent": intent,
		"client_reference_id": id, "amount_total": amount, "currency": "jpy", "metadata": map[string]string{"kind": "consultation", "consultation_id": id}})
	return r.Code
}

func ticketReferralCounted(t *testing.T, purchase string) int {
	t.Helper()
	var n int
	queryRowArgs(t, `SELECT count(*) FROM coaching_referral_counts WHERE ticket_purchase_id=$1`, []any{purchase}, &n)
	return n
}

func referralCodeIssued(t *testing.T, owner string) int {
	t.Helper()
	var n int
	queryRowArgs(t, `SELECT count(*) FROM coaching_referral_codes WHERE owner_id=$1`, []any{owner}, &n)
	return n
}

func ownerOf(t *testing.T, purchase string) string {
	t.Helper()
	var owner string
	queryRowArgs(t, `SELECT owner_id FROM coaching_ticket_purchases WHERE id=$1`, []any{purchase}, &owner)
	return owner
}

// 細目1：JP カードの決済完了では、従来どおりチケットが付与され、紹介の起点も記録される。
func TestTicketPurchaseJPCardCredits(t *testing.T) {
	h, f := setupStripe(t)
	_, tu := demoUser(t, h, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
	id := startTicketPurchase(t, h, tu, "six", uuid.NewString(), "/coaching/new")
	if rec := payTicketPurchaseWith(t, h, id, "pi_ticket_"+id, 2500); rec.Code != 200 {
		t.Fatalf("pay %d %s", rec.Code, rec.Body.String())
	}
	if got := ticketSummary(t, h, tu).Balance.Available; got != 6 {
		t.Fatalf("balance %d", got)
	}
	if len(f.refundList()) != 0 {
		t.Fatalf("refunds %v", f.refundList())
	}
	if referralCodeIssued(t, ownerOf(t, id)) != 1 {
		t.Fatal("referral origin not recorded for JP card")
	}
	assertTicketAccounting(t, h, adminToken(t, h))
}

// 細目2・4・5：非JPカードの決済完了では、使えるチケットは0枚のまま全額の返金が用意され（実行もされ）、
// 紹介の起点は記録されない。同じ完了の再送で二重にならない。その後チケット払いの依頼は作れない。
// 戻り画面の購入照会は、単発と同じ理由の文言を返す。
func TestTicketPurchaseNonJPCardRefundsWithoutCredit(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	admin := adminToken(t, h)
	_, _ = onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)
	// 他人の紹介IDを入れた購入でも、紹介のカウントにしない。
	_, referrer := demoUser(t, h, time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC))
	buySixTickets(t, h, referrer)
	var code string
	queryRowArgs(t, `SELECT code FROM coaching_referral_codes WHERE owner_id=(SELECT owner_id FROM coaching_ticket_purchases ORDER BY created_at LIMIT 1)`, nil, &code)
	r := doJSON(t, h, http.MethodPost, "/v1/coaching/tickets/checkout", tu, map[string]any{"product_id": "six", "request_key": uuid.NewString(), "return_path": "/coaching/new", "guardian_permission": true, "referral_code": code})
	if r.Code != 200 {
		t.Fatalf("checkout %d %s", r.Code, r.Body.String())
	}
	var started struct {
		ID string `json:"purchase_id"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	id := started.ID
	before := len(f.refundList())

	intent := "pi_nonjp_ticket_" + id
	for i := 0; i < 2; i++ {
		if rec := payTicketPurchaseWith(t, h, id, intent, 2500); rec.Code != 200 {
			t.Fatalf("pay %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	state := ticketSummary(t, h, tu)
	if state.Balance.Available != 0 || state.Balance.Reserved != 0 {
		t.Fatalf("credited %+v", state.Balance)
	}
	refunds := f.refundList()[before:]
	if len(refunds) != 1 || refunds[0].Get("payment_intent") != intent || refunds[0].Get("amount") != "2500" {
		t.Fatalf("refunds %v", refunds)
	}
	var rows, quantity, amount int
	var status string
	queryRowArgs(t, `SELECT count(*),COALESCE(sum(quantity),0)::integer,COALESCE(sum(amount_yen),0)::integer FROM coaching_ticket_refunds WHERE purchase_id=$1`, []any{id}, &rows, &quantity, &amount)
	if rows != 1 || quantity != 6 || amount != 2500 {
		t.Fatalf("refund rows %d %d %d", rows, quantity, amount)
	}
	queryRowArgs(t, `SELECT status FROM coaching_ticket_purchases WHERE id=$1`, []any{id}, &status)
	if status != "refunded" {
		t.Fatalf("purchase status %q", status)
	}
	var available, refunded int
	queryRowArgs(t, `SELECT available,refunded FROM coaching_ticket_lots WHERE purchase_id=$1`, []any{id}, &available, &refunded)
	if available != 0 || refunded != 6 {
		t.Fatalf("lot %d %d", available, refunded)
	}
	// 台帳には付与と返金の1行ずつ（再送で増えない）。
	var ledger int
	queryRowArgs(t, `SELECT count(*) FROM coaching_ticket_ledger WHERE purchase_ref=$1`, []any{id}, &ledger)
	if ledger != 2 {
		t.Fatalf("ledger rows %d", ledger)
	}
	if ticketReferralCounted(t, id) != 0 {
		t.Fatal("referral counted for refunded non-JP purchase")
	}
	if referralCodeIssued(t, ownerOf(t, id)) != 0 {
		t.Fatal("referral origin issued for refunded non-JP purchase")
	}
	// 戻り画面の照会は理由の文言を返す。
	rec := doJSON(t, h, http.MethodGet, "/v1/coaching/tickets/purchases/"+id, tu, nil)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "card_country_not_supported") ||
		!strings.Contains(rec.Body.String(), "海外で発行されたカードはご利用いただけません") {
		t.Fatalf("purchase %d %s", rec.Code, rec.Body.String())
	}
	// 細目5：残高0のため、チケット払いの依頼は従来の不足エラーになる。
	body := reviewBodyFor(t, h, tu)
	body["method"] = "tickets"
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, body); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "insufficient_tickets") {
		t.Fatalf("ticket review %d %s", rec.Code, rec.Body.String())
	}
	if len(f.refundList()[before:]) != 1 {
		t.Fatalf("double refund %v", f.refundList())
	}
	assertTicketAccounting(t, h, admin)
}

// 細目2（戻り画面が先）：Webhook より先に戻り画面の照会で確定しても、付与されず理由が返る。
func TestTicketPurchaseNonJPCardOnReturnSync(t *testing.T) {
	h, f := setupStripe(t)
	_, tu := demoUser(t, h, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
	id := startTicketPurchase(t, h, tu, "six", uuid.NewString(), "/coaching/new")
	var session string
	queryRowArgs(t, `SELECT checkout_session_id FROM coaching_ticket_purchases WHERE id=$1`, []any{id}, &session)
	intent := "pi_nonjp_sync_" + id
	f.setCheckoutState(session, map[string]any{"status": "complete", "payment_status": "paid", "payment_intent": intent,
		"client_reference_id": id, "amount_total": 5000, "currency": "jpy", "metadata": map[string]any{"kind": "tickets", "purchase_id": id}})
	rec := doJSON(t, h, http.MethodGet, "/v1/coaching/tickets/purchases/"+id, tu, nil)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "card_country_not_supported") {
		t.Fatalf("purchase %d %s", rec.Code, rec.Body.String())
	}
	if got := ticketSummary(t, h, tu).Balance.Available; got != 0 {
		t.Fatalf("balance %d", got)
	}
	// 後から届く Webhook は素通り（二重返金なし）。
	if rec := payTicketPurchaseWith(t, h, id, intent, 2500); rec.Code != 200 {
		t.Fatalf("webhook %d %s", rec.Code, rec.Body.String())
	}
	if refunds := f.refundList(); len(refunds) != 1 || refunds[0].Get("payment_intent") != intent {
		t.Fatalf("refunds %v", refunds)
	}
}

// 非JPの返金が Stripe で失敗しても、チケットを使える枚数に戻さず運営者の対応待ちにする。
func TestTicketPurchaseNonJPRefundFailureKeepsTicketsUnusable(t *testing.T) {
	h, f := setupStripe(t)
	f.ticketRefundStatus = "failed"
	_, tu := demoUser(t, h, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
	id := startTicketPurchase(t, h, tu, "six", uuid.NewString(), "/coaching/new")
	if rec := payTicketPurchaseWith(t, h, id, "pi_nonjp_fail_"+id, 2500); rec.Code != 200 {
		t.Fatalf("pay %d %s", rec.Code, rec.Body.String())
	}
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if got := ticketSummary(t, h, tu).Balance.Available; got != 0 {
		t.Fatalf("tickets returned after failed refund: %d", got)
	}
	var status string
	queryRowArgs(t, `SELECT status FROM coaching_ticket_refunds WHERE purchase_id=$1`, []any{id}, &status)
	if status != "needs_action" {
		t.Fatalf("refund status %q", status)
	}
	assertTicketAccounting(t, h, adminToken(t, h))
}

// 細目3：発行国が取れない決済完了は、従来どおり付与される。
func TestTicketPurchaseUnknownCountryCredits(t *testing.T) {
	h, f := setupStripe(t)
	_, tu := demoUser(t, h, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
	id := startTicketPurchase(t, h, tu, "six", uuid.NewString(), "/coaching/new")
	if rec := payTicketPurchaseWith(t, h, id, "pi_unknown_ticket_"+id, 2500); rec.Code != 200 {
		t.Fatalf("pay %d %s", rec.Code, rec.Body.String())
	}
	if got := ticketSummary(t, h, tu).Balance.Available; got != 6 {
		t.Fatalf("balance %d", got)
	}
	if len(f.refundList()) != 0 {
		t.Fatalf("refunds %v", f.refundList())
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/coaching/tickets/purchases/"+id, tu, nil); rec.Code != 200 {
		t.Fatalf("purchase %d %s", rec.Code, rec.Body.String())
	}
}

func consultationCoachNotices(t *testing.T, consultation string) int {
	t.Helper()
	var n int
	queryRowArgs(t, `SELECT count(*) FROM notifications WHERE recipient_id=(SELECT coach_id FROM coaching_consultations WHERE id=$1) AND title LIKE '追加%が届きました'`, []any{consultation}, &n)
	return n
}

// 細目6：依頼側の追加質問を JP カードで払うと、従来どおり成立し、コーチに通知が届く。
func TestReviewConsultationJPCardDelivered(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	owner, _, root := answeredTicketRoom(t, h)
	c := consultationDraft(t, h, owner, "/v1/coaching/requests/"+root+"/consultations", uuid.NewString(), "次の大会ではどこを意識しますか")
	consultationAction(t, h, owner, c.ID, "submit", nil)
	if code := payConsultationWith(t, h, c.ID, "pi_consult_"+c.ID, 500); code != 200 {
		t.Fatalf("pay %d", code)
	}
	var status, state string
	queryRowArgs(t, `SELECT status,payment_state FROM coaching_consultations WHERE id=$1`, []any{c.ID}, &status, &state)
	if status != "offered" || state != "paid" {
		t.Fatalf("consultation %s %s", status, state)
	}
	if consultationCoachNotices(t, c.ID) != 1 {
		t.Fatal("coach not notified")
	}
	if len(f.refundList()) != 0 {
		t.Fatalf("refunds %v", f.refundList())
	}
}

// assertNonJPConsultationRefunded は、非JPカードの支払いで相談が成立せず、コーチに届かず、全額が返金されたことを確かめる。
func assertNonJPConsultationRefunded(t *testing.T, h http.Handler, f *fakeStripe, id, intent, amount string, coach string) {
	t.Helper()
	var status, state string
	queryRowArgs(t, `SELECT status,payment_state FROM coaching_consultations WHERE id=$1`, []any{id}, &status, &state)
	if status != "expired" || (state != "refund_pending" && state != "refunded") {
		t.Fatalf("consultation %s %s", status, state)
	}
	if consultationCoachNotices(t, id) != 0 {
		t.Fatal("coach notified for non-JP card")
	}
	if r := doJSON(t, h, "POST", "/v1/coaching/consultations/"+id+"/accept", coach, nil); r.Code == 200 {
		t.Fatal("coach could accept non-JP consultation")
	}
	for i := 0; i < 2; i++ {
		if r := tick(t, h, "tick-secret"); r.Code != 200 {
			t.Fatal(r.Body.String())
		}
	}
	queryRowArgs(t, `SELECT payment_state FROM coaching_consultations WHERE id=$1`, []any{id}, &state)
	if state != "refunded" {
		t.Fatalf("payment state %s", state)
	}
	var refunds int
	queryRowArgs(t, `SELECT count(*) FROM coaching_consultation_refunds WHERE consultation_id=$1`, []any{id}, &refunds)
	if refunds != 1 {
		t.Fatalf("refund rows %d", refunds)
	}
	list := f.refundList()
	if len(list) != 1 || list[0].Get("payment_intent") != intent || list[0].Get("amount") != amount {
		t.Fatalf("refunds %v", list)
	}
}

// 細目7：依頼側の追加質問を非JPカードで払うと、成立せず、コーチに届かず通知もなく、全額が返金される。
func TestReviewConsultationNonJPCardRefunded(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	owner, coach, root := answeredTicketRoom(t, h)
	c := consultationDraft(t, h, owner, "/v1/coaching/requests/"+root+"/consultations", uuid.NewString(), "次の大会ではどこを意識しますか")
	consultationAction(t, h, owner, c.ID, "submit", nil)
	intent := "pi_nonjp_consult_" + c.ID
	for i := 0; i < 2; i++ {
		if code := payConsultationWith(t, h, c.ID, intent, 500); code != 200 {
			t.Fatalf("pay %d: %d", i, code)
		}
	}
	assertNonJPConsultationRefunded(t, h, f, c.ID, intent, "500", coach)
	if offers := coachConsultationCount(t, c.ID); offers != 0 {
		t.Fatalf("earnings %d", offers)
	}
}

func coachConsultationCount(t *testing.T, id string) int {
	t.Helper()
	var n int
	queryRowArgs(t, `SELECT count(*) FROM coach_earnings WHERE consultation_id=$1`, []any{id}, &n)
	return n
}

// 細目8：発行国が取れない場合は従来どおり成立する。同じ完了の再送で二重成立・二重返金にならない。
func TestReviewConsultationUnknownCountryDelivered(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	owner, _, root := answeredTicketRoom(t, h)
	c := consultationDraft(t, h, owner, "/v1/coaching/requests/"+root+"/consultations", uuid.NewString(), "次の大会ではどこを意識しますか")
	consultationAction(t, h, owner, c.ID, "submit", nil)
	for i := 0; i < 2; i++ {
		if code := payConsultationWith(t, h, c.ID, "pi_unknown_consult_"+c.ID, 500); code != 200 {
			t.Fatalf("pay %d: %d", i, code)
		}
	}
	var status, state string
	queryRowArgs(t, `SELECT status,payment_state FROM coaching_consultations WHERE id=$1`, []any{c.ID}, &status, &state)
	if status != "offered" || state != "paid" {
		t.Fatalf("consultation %s %s", status, state)
	}
	if n := consultationCoachNotices(t, c.ID); n != 1 {
		t.Fatalf("coach notices %d", n)
	}
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if len(f.refundList()) != 0 {
		t.Fatalf("refunds %v", f.refundList())
	}
}

// 細目9：契約側の追加相談でも、非JPカードは成立しない。
func TestContractConsultationNonJPCardRefunded(t *testing.T) {
	h, f := setupStripe(t)
	owner, coach, root := endedCashRoom(t, h)
	c := consultationDraft(t, h, owner, "/v1/coaching/contracts/"+root+"/consultations", uuid.NewString(), "契約終了後の相談")
	consultationAction(t, h, owner, c.ID, "submit", nil)
	consultationAction(t, h, owner, c.ID, "checkout", nil)
	intent := "pi_nonjp_contract_" + c.ID
	for i := 0; i < 2; i++ {
		if code := payConsultationWith(t, h, c.ID, intent, 1200); code != 200 {
			t.Fatalf("pay %d: %d", i, code)
		}
	}
	assertNonJPConsultationRefunded(t, h, f, c.ID, intent, "1200", coach)
}
