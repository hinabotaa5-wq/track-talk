package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 非JPカードの仮押さえ取消し（本体 #796）の受け入れテスト。
// 対象は単発1回プランのカード払い（仮押さえする唯一の経路）のみ。
// 追加質問・チケット購入・継続プランは即時確定のため仮押さえがなく対象外。

// createCardReview は、カード払いの依頼を作り、支払い画面まで進める。
// Webhook は送らない（支払いの到着は各テストで送る）。
func createCardReview(t *testing.T, h http.Handler, tu string) (id, session string) {
	t.Helper()
	body := reviewBodyFor(t, h, tu)
	body["method"] = "card"
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		reviewRequestView
		CheckoutURL string `json:"checkout_url"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Status != "awaiting_payment" || created.CheckoutURL == "" {
		t.Fatalf("created %+v", created)
	}
	queryRowArgs(t, `SELECT checkout_session_id FROM review_payments WHERE request_id = $1`, []any{created.ID}, &session)
	return created.ID, session
}

func completeCardWebhook(t *testing.T, h http.Handler, session, intent string) *httptest.ResponseRecorder {
	t.Helper()
	return stripeWebhook(t, h, testWebhookSecret, "checkout.session.completed", map[string]any{
		"id": session, "payment_status": "unpaid", "payment_intent": intent, "metadata": map[string]any{"kind": "review"},
	})
}

// TestReviewPaymentNonJPCardCanceled は細目1・6：非JPカードでは仮押さえを
// 取り消し、依頼を作らず、コーチに割り当てない。再送では何も起きない。
func TestReviewPaymentNonJPCardCanceled(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	admin := adminToken(t, h)
	_, tc := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)

	id, session := createCardReview(t, h, tu)

	// 1回目は理由の文言を返す（Webhook は再送で収束する）。
	rec := completeCardWebhook(t, h, session, "pi_nonjp_1")
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "card_country_not_supported") {
		t.Fatalf("webhook %d %s", rec.Code, rec.Body.String())
	}
	if _, canceled := f.calls(); len(canceled) != 1 || canceled[0] != "pi_nonjp_1" {
		t.Fatalf("canceled %+v", canceled)
	}
	if got := reviewPaymentStatus(t, id); got != "canceled" {
		t.Fatalf("payment status %q, want canceled", got)
	}
	var reqStatus string
	queryRowArgs(t, `SELECT status FROM review_requests WHERE id = $1`, []any{id}, &reqStatus)
	if reqStatus != "awaiting_payment" {
		t.Fatalf("request status %q, want awaiting_payment", reqStatus)
	}
	if offers := coachRequests(t, h, tc).Reviewing; len(offers) != 0 {
		t.Fatalf("assigned to coach %+v", offers)
	}

	// 同じ知らせの再送では何も起きない（二重の取り消しなし・200）。
	rec = completeCardWebhook(t, h, session, "pi_nonjp_1")
	if rec.Code != http.StatusOK {
		t.Fatalf("redeliver %d %s", rec.Code, rec.Body.String())
	}
	if _, canceled := f.calls(); len(canceled) != 1 {
		t.Fatalf("double cancel %+v", canceled)
	}
}

// TestReviewPaymentNonJPReasonReturnedOnSync は細目1（文言）：戻り画面の sync
// では、理由が 422 の文言で返る。
func TestReviewPaymentNonJPReasonReturnedOnSync(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	admin := adminToken(t, h)
	_, _ = onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)

	id, session := createCardReview(t, h, tu)
	f.setCheckoutState(session, map[string]any{
		"status": "complete", "payment_status": "unpaid", "payment_intent": "pi_nonjp_9",
	})
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/payment/sync", tu, nil)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("sync %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "card_country_not_supported") ||
		!strings.Contains(rec.Body.String(), "海外で発行されたカードはご利用いただけません") {
		t.Fatalf("sync body %s", rec.Body.String())
	}
	if got := reviewPaymentStatus(t, id); got != "canceled" {
		t.Fatalf("payment status %q, want canceled", got)
	}
	if _, canceled := f.calls(); len(canceled) != 1 || canceled[0] != "pi_nonjp_9" {
		t.Fatalf("canceled %+v", canceled)
	}
}

// TestReviewPaymentUnknownCountryProceeds は細目3：発行国が取れないときは
// ブロックせず、従来どおり仮押さえして割り当てる。
func TestReviewPaymentUnknownCountryProceeds(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	admin := adminToken(t, h)
	_, tc := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)

	id, session := createCardReview(t, h, tu)
	rec := completeCardWebhook(t, h, session, "pi_unknown_1")
	if rec.Code != http.StatusOK {
		t.Fatalf("webhook %d %s", rec.Code, rec.Body.String())
	}
	if _, canceled := f.calls(); len(canceled) != 0 {
		t.Fatalf("canceled %+v", canceled)
	}
	if got := reviewPaymentStatus(t, id); got != "authorized" {
		t.Fatalf("payment status %q, want authorized", got)
	}
	if offers := coachRequests(t, h, tc).Reviewing; len(offers) != 1 || offers[0].ID != id {
		t.Fatalf("offers %+v", offers)
	}
}

// TestReviewPaymentRetryWithJPAfterNonJPBlock は細目4：取り消し後に同じ依頼へ
// JP カードで申し込むと、通常どおり成立する（状態の残留なし）。
func TestReviewPaymentRetryWithJPAfterNonJPBlock(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	_, tc := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)

	id, session := createCardReview(t, h, tu)
	if rec := completeCardWebhook(t, h, session, "pi_nonjp_2"); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("block %d %s", rec.Code, rec.Body.String())
	}
	if got := reviewPaymentStatus(t, id); got != "canceled" {
		t.Fatalf("blocked payment status %q, want canceled", got)
	}

	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/checkout", tu,
		map[string]any{"method": "card", "guardian_permission": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("checkout again %d %s", rec.Code, rec.Body.String())
	}
	var retrySession string
	queryRowArgs(t, `SELECT checkout_session_id FROM review_payments WHERE request_id = $1 AND status = 'pending' ORDER BY created_at DESC LIMIT 1`,
		[]any{id}, &retrySession)
	if retrySession == "" || retrySession == session {
		t.Fatalf("no new pending payment %q", retrySession)
	}
	if rec := completeCardWebhook(t, h, retrySession, "pi_card_retry_1"); rec.Code != http.StatusOK {
		t.Fatalf("webhook %d %s", rec.Code, rec.Body.String())
	}
	var retryStatus string
	queryRowArgs(t, `SELECT status FROM review_payments WHERE request_id = $1 AND status <> 'pending' ORDER BY created_at DESC LIMIT 1`,
		[]any{id}, &retryStatus)
	if retryStatus != "authorized" {
		t.Fatalf("retry payment status %q, want authorized", retryStatus)
	}
	if offers := coachRequests(t, h, tc).Reviewing; len(offers) != 1 || offers[0].ID != id {
		t.Fatalf("offers %+v", offers)
	}
}

// TestTicketReviewUnaffectedByNonJPBlock は細目6：チケットで支払う申込みは
// カード判定の影響を受けず、従来どおり割り当てられる。Stripe も触らない。
func TestTicketReviewUnaffectedByNonJPBlock(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	admin := adminToken(t, h)
	_, tc := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)
	buySixTickets(t, h, tu)

	id := ticketReview(t, h, tu)

	if offers := coachRequests(t, h, tc).Reviewing; len(offers) != 1 || offers[0].ID != id {
		t.Fatalf("offers %+v", offers)
	}
	if captured, canceled := f.calls(); len(captured) != 0 || len(canceled) != 0 {
		t.Fatalf("stripe touched captured=%v canceled=%v", captured, canceled)
	}
}
