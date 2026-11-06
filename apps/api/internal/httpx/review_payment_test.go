package httpx_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/httpx"
)

// allowPayPayForTest は、このテストの間だけ、動画添削とその追加相談の支払い方法に PayPay を戻す。
// 今は PayPay を受け付けていないが、その場で払い・目安を過ぎたら返金する経路は残しているので、
// 再び使えるようにした時に備えてテストで確かめ続ける。
func allowPayPayForTest(t *testing.T) {
	t.Helper()
	t.Cleanup(httpx.SetReviewPaymentMethodsForTest([]string{"card", "paypay"}))
}

// createReviewWith は、依頼を送る。PayPay を選んだときは、この1回の申込みの間だけ PayPay を受け付ける。
// 後の支払い画面の作り直しなどでも PayPay を使うテストは、allowPayPayForTest でテスト全体を通して受け付ける。
func createReviewWith(t *testing.T, h http.Handler, tu, method string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	if method == "paypay" {
		defer httpx.SetReviewPaymentMethodsForTest([]string{"card", "paypay"})()
	}
	return doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, body)
}

// paidReview は、支払い方法を選んで依頼を送り、Stripe から支払いが届いたところまで進める。
// PayPay を選んだときは、この申込みの間だけ PayPay を受け付ける（PayPay を受け付けていた時の経路を確かめるため）。
func paidReview(t *testing.T, h http.Handler, tu, method, intent string) string {
	t.Helper()
	body := reviewBodyFor(t, h, tu)
	body["method"] = method
	rec := createReviewWith(t, h, tu, method, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		reviewRequestView
		CheckoutURL string `json:"checkout_url"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.Status != "awaiting_payment" || created.CheckoutURL == "" {
		t.Fatalf("created %+v", created)
	}
	status := "paid"
	if method == "card" {
		status = "unpaid" // 仮押さえのときは unpaid のまま届く。
	}
	var session string
	queryRowArgs(t, `SELECT checkout_session_id FROM review_payments WHERE request_id = $1`, []any{created.ID}, &session)
	rec = stripeWebhook(t, h, testWebhookSecret, "checkout.session.completed", map[string]any{
		"id": session, "payment_status": status, "payment_intent": intent, "metadata": map[string]any{"kind": "review"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("webhook %d %s", rec.Code, rec.Body.String())
	}
	return created.ID
}

func reviewPaymentStatus(t *testing.T, id string) string {
	t.Helper()
	var status string
	queryRowArgs(t, `SELECT status FROM review_payments WHERE request_id = $1 AND status <> 'pending'`, []any{id}, &status)
	return status
}

func TestReviewPaymentCapturedOnReturn(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	tadmin := adminToken(t, h)
	_, tc := onDutyCoach(t, h, tadmin)
	_, tu := demoRequester(t, h, tadmin)

	rec := doJSON(t, h, http.MethodGet, "/v1/coaching/estimate?tier=coach&event=100m", tu, nil)
	if !strings.Contains(rec.Body.String(), `"payments":true`) || !strings.Contains(rec.Body.String(), `"price_yen":1500`) {
		t.Fatalf("estimate %s", rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, reviewBodyFor(t, h, tu)); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"method"`) {
		t.Fatalf("no method %d %s", rec.Code, rec.Body.String())
	}

	id := paidReview(t, h, tu, "card", "pi_card_1")
	form := f.lastSession()
	if form.Get("payment_method_types[0]") != "card" || form.Get("payment_intent_data[capture_method]") != "manual" ||
		form.Get("line_items[0][price_data][unit_amount]") != "1500" || form.Get("metadata[kind]") != "review" ||
		form.Get("success_url") != "https://web.test/coaching/requests/"+id+"?paid=1" {
		t.Fatalf("session %v", form)
	}
	if reviewPaymentStatus(t, id) != "authorized" {
		t.Fatalf("authorized %s", reviewPaymentStatus(t, id))
	}
	// 支払いが届いたので、コーチに打診される。
	offers := coachRequests(t, h, tc).Offers
	if len(offers) != 1 || offers[0].Request.ID != id {
		t.Fatalf("offers %+v", offers)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/offers/"+offers[0].ID+"/accept", tc, nil); rec.Code != http.StatusOK {
		t.Fatalf("accept %d", rec.Code)
	}
	if captured, _ := f.calls(); len(captured) != 0 {
		t.Fatalf("captured before return %v", captured)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", tc, responseBody()); rec.Code != http.StatusOK {
		t.Fatalf("response %d %s", rec.Code, rec.Body.String())
	}
	// 返却で確定する。
	if captured, _ := f.calls(); len(captured) != 1 || captured[0] != "pi_card_1" {
		t.Fatalf("captured %v", captured)
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/coaching/requests/"+id, tu, nil)
	if !strings.Contains(rec.Body.String(), `"payment":{"amount_yen":1500`) || !strings.Contains(rec.Body.String(), `"status":"captured"`) {
		t.Fatalf("detail %s", rec.Body.String())
	}
}

func TestReviewPaymentReleasedWhenOverdue(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	tadmin := adminToken(t, h)
	_, tu := demoRequester(t, h, tadmin)

	// 受付中のコーチがいないので、どちらも待ち行列で止まる。
	card := paidReview(t, h, tu, "card", "pi_card_2")
	paypay := paidReview(t, h, tu, "paypay", "pi_paypay_2")
	if reviewPaymentStatus(t, paypay) != "captured" {
		t.Fatalf("paypay %s", reviewPaymentStatus(t, paypay))
	}
	// 払わないまま閉じた依頼。
	body := reviewBodyFor(t, h, tu)
	body["method"] = "card"
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, body)
	var unpaid reviewRequestView
	_ = json.Unmarshal(rec.Body.Bytes(), &unpaid)
	// 今は PayPay を受け付けないので、支払い画面の作り直しでも PayPay は断る。
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+unpaid.ID+"/checkout", tu, map[string]any{"method": "paypay"}); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"method"`) {
		t.Fatalf("paypay checkout must be rejected %d %s", rec.Code, rec.Body.String())
	}
	// 以下は PayPay を再び使えるようにした時のふるまい。
	allowPayPayForTest(t)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+unpaid.ID+"/checkout", tu, map[string]any{"method": "paypay"}); rec.Code != http.StatusOK {
		t.Fatalf("checkout again %d %s", rec.Code, rec.Body.String())
	}

	execSQLArgs(t, `UPDATE review_requests SET due_at = now() - interval '1 minute' WHERE id = ANY($1::uuid[])`, []string{card, paypay})
	execSQLArgs(t, `UPDATE review_payments SET authorized_at = now() - interval '2 minutes' WHERE request_id = ANY($1::uuid[])`, []string{card, paypay})
	execSQLArgs(t, `UPDATE review_requests SET created_at = now() - interval '3 hours' WHERE id = $1`, unpaid.ID)
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"review_payments_released":2`) {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
	if _, canceled := f.calls(); len(canceled) != 1 || canceled[0] != "pi_card_2" {
		t.Fatalf("canceled %v", canceled)
	}
	refunds := f.refundList()
	if len(refunds) != 1 || refunds[0].Get("payment_intent") != "pi_paypay_2" || refunds[0].Get("amount") != "1500" {
		t.Fatalf("refunds %v", refunds)
	}
	if reviewPaymentStatus(t, card) != "canceled" || reviewPaymentStatus(t, paypay) != "refunded" {
		t.Fatalf("statuses %s %s", reviewPaymentStatus(t, card), reviewPaymentStatus(t, paypay))
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/coaching/requests/"+unpaid.ID, tu, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("unpaid request should be gone %d", rec.Code)
	}

	// 目安を過ぎてから届いた支払いは、始めずに取り消す。
	late := reviewBodyFor(t, h, tu)
	late["method"] = "card"
	rec = doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, late)
	var lateReq reviewRequestView
	_ = json.Unmarshal(rec.Body.Bytes(), &lateReq)
	execSQLArgs(t, `UPDATE review_requests SET due_at = now() - interval '1 minute' WHERE id = $1`, lateReq.ID)
	var session string
	queryRowArgs(t, `SELECT checkout_session_id FROM review_payments WHERE request_id = $1`, []any{lateReq.ID}, &session)
	stripeWebhook(t, h, testWebhookSecret, "checkout.session.completed", map[string]any{
		"id": session, "payment_status": "unpaid", "payment_intent": "pi_card_late", "metadata": map[string]any{"kind": "review"},
	})
	if _, canceled := f.calls(); len(canceled) != 2 || canceled[1] != "pi_card_late" {
		t.Fatalf("late canceled %v", canceled)
	}
}

// 退会すると依頼が消える。確定していない代金があるあいだは退会させず、返却済みで確定した代金は退会しても返金しない。
func TestWithdrawKeepsSettledReviewPayment(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	tadmin := adminToken(t, h)
	_, tu := demoRequester(t, h, tadmin)
	confirm := map[string]string{"confirm": "退会する"}

	id := paidReview(t, h, tu, "paypay", "pi_paypay_withdraw")
	if rec := doJSON(t, h, http.MethodDelete, "/v1/me", tu, confirm); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "review_in_progress") {
		t.Fatalf("withdraw before return %d %s", rec.Code, rec.Body.String())
	}

	execSQLArgs(t, `UPDATE review_requests SET status = 'returned', returned_at = now() WHERE id = $1`, id)
	execSQLArgs(t, `UPDATE review_payments SET authorized_at = now() - interval '2 minutes' WHERE request_id = $1`, id)
	if rec := doJSON(t, h, http.MethodDelete, "/v1/me", tu, confirm); rec.Code != http.StatusOK {
		t.Fatalf("withdraw after return %d %s", rec.Code, rec.Body.String())
	}
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
	if refunds := f.refundList(); len(refunds) != 0 {
		t.Fatalf("settled payment refunded %v", refunds)
	}
}

// 1本の依頼に二重の支払いがあったら、返却のあとでも最初の1本だけを確定し、残りは取り消す。
func TestReviewPaymentDuplicateAfterReturn(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	tadmin := adminToken(t, h)
	_, tu := demoRequester(t, h, tadmin)
	id := paidReview(t, h, tu, "card", "pi_card_first")
	execSQLArgs(t, `INSERT INTO review_payments (id, request_id, payer_id, method, amount_yen, checkout_session_id, checkout_url,
			session_expires_at, payment_intent_id, status, authorized_at)
		SELECT gen_random_uuid(), request_id, payer_id, method, amount_yen, 'cs_second', checkout_url,
			session_expires_at, 'pi_card_second', 'authorized', now()
		FROM review_payments WHERE request_id = $1 AND status = 'authorized'`, id)
	execSQLArgs(t, `UPDATE review_payments SET authorized_at = authorized_at - interval '5 minutes' WHERE payment_intent_id = 'pi_card_first'`)
	execSQLArgs(t, `UPDATE review_payments SET authorized_at = authorized_at - interval '2 minutes' WHERE payment_intent_id = 'pi_card_second'`)
	execSQLArgs(t, `UPDATE review_requests SET status = 'returned', returned_at = now() WHERE id = $1`, id)
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
	captured, canceled := f.calls()
	if len(captured) != 1 || captured[0] != "pi_card_first" || len(canceled) != 1 || canceled[0] != "pi_card_second" {
		t.Fatalf("captured %v canceled %v", captured, canceled)
	}
}

// Stripe が断り続ける支払いが50件以上たまっても、ほかの支払いは次の tick で処理される。
func TestReviewPaymentReleaseDoesNotStallBehindFailures(t *testing.T) {
	h, f := setupStripe(t)
	payer := uuid.New()
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", token(t, payer), nil); rec.Code != http.StatusOK {
		t.Fatalf("me %d", rec.Code)
	}
	// 依頼が消えた仮押さえ（取り消す対象）。断られる50件を先に、正常な1件をあとに入れる。
	insert := `INSERT INTO review_payments (id, payer_id, method, amount_yen, checkout_session_id, checkout_url, session_expires_at, payment_intent_id, status, authorized_at, updated_at)
		VALUES (gen_random_uuid(), $1, 'card', 1000, $2, 'https://checkout.stripe.test/x', now(), $2, 'authorized', now() - interval '1 hour', $3)`
	// 断られる行が残るとほかのテストの tick の枠を使うので、終わったら消す。
	t.Cleanup(func() { execSQLArgs(t, `DELETE FROM review_payments WHERE payer_id = $1`, payer) })
	for i := 0; i < 50; i++ {
		execSQLArgs(t, insert, payer, fmt.Sprintf("pi_fail_%s_%d", payer, i), time.Now().Add(-time.Hour))
	}
	good := "pi_good_" + payer.String()
	execSQLArgs(t, insert, payer, good, time.Now())
	canceled := func() bool {
		_, c := f.calls()
		return slices.Contains(c, good)
	}
	for range 2 {
		if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK {
			t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
		}
		if canceled() {
			return
		}
	}
	t.Fatal("the good payment was never released")
}

// 18歳未満の支払い画面は、ロックを持つトランザクションの接続だけで作る。
// 別の接続を待つと、接続の数だけ同時に来たときに全員が止まる。
func TestReviewPaymentMinorCheckoutUsesOneConnection(t *testing.T) {
	allowPayPayForTest(t)
	testPoolMaxConns = 1
	t.Cleanup(func() { testPoolMaxConns = 0 })
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	_, tm := demoUser(t, h, time.Date(time.Now().Year()-14, 1, 1, 0, 0, 0, 0, time.UTC))
	body := reviewBodyFor(t, h, tm)
	body["method"] = "paypay"
	body["guardian_permission"] = true
	done := make(chan int, 1)
	go func() { done <- doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tm, body).Code }()
	select {
	case code := <-done:
		if code != http.StatusCreated {
			t.Fatalf("create %d", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("minor checkout waited for a second connection while holding the lock")
	}
}

// 返却して代金が確定した直後(tick が報酬を記録する前)に依頼した人が退会しても、コーチの報酬は残る。
func TestWithdrawRecordsCoachEarningFirst(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	tadmin := adminToken(t, h)
	coach, _ := onDutyCoach(t, h, tadmin)
	_, tu := demoRequester(t, h, tadmin)
	id := paidReview(t, h, tu, "paypay", "pi_paypay_earning")
	execSQLArgs(t, `UPDATE review_requests SET status = 'returned', returned_at = now(), coach_id = $2 WHERE id = $1`, id, coach)
	execSQLArgs(t, `UPDATE review_payments SET authorized_at = now() - interval '2 minutes' WHERE request_id = $1`, id)
	if rec := doJSON(t, h, http.MethodDelete, "/v1/me", tu, map[string]string{"confirm": "退会する"}); rec.Code != http.StatusOK {
		t.Fatalf("withdraw %d %s", rec.Code, rec.Body.String())
	}
	var n int
	queryRowArgs(t, `SELECT count(*)::int FROM coach_earnings WHERE coach_id = $1`, []any{coach}, &n)
	if n != 1 {
		t.Fatalf("coach earnings after the requester withdrew = %d", n)
	}
}
