package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// paidReview は、支払い方法を選んで依頼を送り、Stripe から支払いが届いたところまで進める。
func paidReview(t *testing.T, h http.Handler, tu, method, intent string) string {
	t.Helper()
	body := reviewBody()
	body["method"] = method
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, body)
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
	if !strings.Contains(rec.Body.String(), `"payments":true`) || !strings.Contains(rec.Body.String(), `"price_yen":1000`) {
		t.Fatalf("estimate %s", rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, reviewBody()); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"method"`) {
		t.Fatalf("no method %d %s", rec.Code, rec.Body.String())
	}

	id := paidReview(t, h, tu, "card", "pi_card_1")
	form := f.lastSession()
	if form.Get("payment_method_types[0]") != "card" || form.Get("payment_intent_data[capture_method]") != "manual" ||
		form.Get("line_items[0][price_data][unit_amount]") != "1000" || form.Get("metadata[kind]") != "review" ||
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
	if !strings.Contains(rec.Body.String(), `"payment":{"amount_yen":1000`) || !strings.Contains(rec.Body.String(), `"status":"captured"`) {
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
	body := reviewBody()
	body["method"] = "card"
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, body)
	var unpaid reviewRequestView
	_ = json.Unmarshal(rec.Body.Bytes(), &unpaid)
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
	if len(refunds) != 1 || refunds[0].Get("payment_intent") != "pi_paypay_2" || refunds[0].Get("amount") != "1000" {
		t.Fatalf("refunds %v", refunds)
	}
	if reviewPaymentStatus(t, card) != "canceled" || reviewPaymentStatus(t, paypay) != "refunded" {
		t.Fatalf("statuses %s %s", reviewPaymentStatus(t, card), reviewPaymentStatus(t, paypay))
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/coaching/requests/"+unpaid.ID, tu, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("unpaid request should be gone %d", rec.Code)
	}

	// 目安を過ぎてから届いた支払いは、始めずに取り消す。
	late := reviewBody()
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

func TestReviewPaymentMinorMonthlyLimit(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	tm, _ := guardianMinor(t, h)
	execSQLArgs(t, `UPDATE guardian_consents SET monthly_limit_yen = 1000`)
	paidReview(t, h, tm, "card", "pi_card_minor")
	body := reviewBody()
	body["method"] = "paypay"
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tm, body)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "monthly_limit") || !strings.Contains(rec.Body.String(), "あと0円") {
		t.Fatalf("over limit %d %s", rec.Code, rec.Body.String())
	}
	body["tier"] = "senpai"
	execSQLArgs(t, `UPDATE guardian_consents SET monthly_limit_yen = 3000`)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tm, body); rec.Code != http.StatusCreated {
		t.Fatalf("within limit %d %s", rec.Code, rec.Body.String())
	}
}
