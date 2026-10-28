package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

func setupTicketSalesClosed(t *testing.T) (http.Handler, *fakeStripe) {
	t.Helper()
	return setupStripeConfig(t, func(cfg *config.Config) { cfg.CoachingTicketSales = false })
}

func sessionCount(f *fakeStripe) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sessions)
}

// チケットの新規販売を止めると、商品が出ず、購入の API を直接呼んでも支払い画面は作られない。
func TestTicketSalesClosedStopsNewPurchases(t *testing.T) {
	h, f := setupTicketSalesClosed(t)
	_, tu := demoRequester(t, h, adminToken(t, h))

	rec := doJSON(t, h, http.MethodGet, "/v1/me/coaching/tickets", tu, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("tickets %d %s", rec.Code, rec.Body.String())
	}
	var summary struct {
		Products    []any `json:"products"`
		CanPurchase bool  `json:"can_purchase"`
		SalesOpen   bool  `json:"sales_open"`
		Reason      string
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.CanPurchase || summary.SalesOpen || len(summary.Products) != 0 || !strings.Contains(rec.Body.String(), "新規販売は終了しました") {
		t.Fatalf("summary %s", rec.Body.String())
	}
	for _, product := range []string{"single", "five"} {
		rec = doJSON(t, h, http.MethodPost, "/v1/coaching/tickets/checkout", tu, map[string]any{"product_id": product, "request_key": uuid.NewString(), "return_path": "/coaching/new"})
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "ticket_sales_closed") {
			t.Fatalf("checkout %s %d %s", product, rec.Code, rec.Body.String())
		}
	}
	var purchases int
	queryRowArgs(t, `SELECT count(*) FROM coaching_ticket_purchases`, nil, &purchases)
	if purchases != 0 || sessionCount(f) != 0 {
		t.Fatalf("purchases %d sessions %d", purchases, sessionCount(f))
	}
}

// チケットを持っていない人は、添削をチケットでは払えず、カードか PayPay の都度払いで申し込む。
func TestTicketSalesClosedReviewIsPayPerUse(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupTicketSalesClosed(t)
	tadmin := adminToken(t, h)
	_, tc := onDutyCoach(t, h, tadmin)
	_, tu := demoRequester(t, h, tadmin)

	body := reviewBodyFor(t, h, tu)
	body["method"] = "tickets"
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, body); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "insufficient_tickets") {
		t.Fatalf("tickets %d %s", rec.Code, rec.Body.String())
	}
	// PayPay は申込み時にその場で払う。支払いが届くまでコーチには回らない。
	body = reviewBodyFor(t, h, tu)
	body["method"] = "paypay"
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID          string
		Status      string
		CheckoutURL string `json:"checkout_url"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	form := f.lastSession()
	if created.Status != "awaiting_payment" || form.Get("payment_method_types[0]") != "paypay" || form.Get("payment_method_types[1]") != "" ||
		form.Get("payment_intent_data[capture_method]") != "" || form.Get("line_items[0][price_data][unit_amount]") != "1500" {
		t.Fatalf("created %+v session %v", created, form)
	}
	if offers := coachRequests(t, h, tc).Offers; len(offers) != 0 {
		t.Fatalf("offered before payment %+v", offers)
	}
	// 承認せずに戻った（支払いが済んでいない知らせ）ときは、依頼は支払い待ちのまま開き直せる。
	var session string
	queryRowArgs(t, `SELECT checkout_session_id FROM review_payments WHERE request_id=$1`, []any{created.ID}, &session)
	if rec := stripeWebhook(t, h, testWebhookSecret, "checkout.session.completed", map[string]any{"id": session, "payment_status": "unpaid", "payment_intent": "pi_paypay_open", "metadata": map[string]any{"kind": "review"}}); rec.Code != http.StatusOK {
		t.Fatalf("unpaid webhook %d", rec.Code)
	}
	if offers := coachRequests(t, h, tc).Offers; len(offers) != 0 {
		t.Fatalf("offered while unpaid %+v", offers)
	}
	reopen := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+created.ID+"/checkout", tu, map[string]any{"method": "card"})
	if reopen.Code != http.StatusOK || !strings.Contains(reopen.Body.String(), "checkout_url") {
		t.Fatalf("reopen %d %s", reopen.Code, reopen.Body.String())
	}
	if form := f.lastSession(); form.Get("payment_method_types[0]") != "card" || form.Get("payment_intent_data[capture_method]") != "manual" {
		t.Fatalf("card session %v", form)
	}
	// PayPay で払い終えた知らせが2回届いても、割り当ては1回だけ。
	paid := map[string]any{"id": session, "payment_status": "paid", "payment_intent": "pi_paypay_done", "metadata": map[string]any{"kind": "review"}}
	for i := 0; i < 2; i++ {
		if rec := stripeWebhook(t, h, testWebhookSecret, "checkout.session.completed", paid); rec.Code != http.StatusOK {
			t.Fatalf("paid webhook %d", rec.Code)
		}
	}
	if offers := coachRequests(t, h, tc).Offers; len(offers) != 1 || offers[0].Request.ID != created.ID {
		t.Fatalf("offers %+v", offers)
	}
	var requests, payments int
	queryRowArgs(t, `SELECT count(*) FROM review_requests`, nil, &requests)
	queryRowArgs(t, `SELECT count(*) FROM review_payments WHERE status='captured'`, nil, &payments)
	if requests != 1 || payments != 1 {
		t.Fatalf("requests %d payments %d", requests, payments)
	}
}

// チケットを持っていない人の、フォーム添削の追加相談は都度払い（カード・PayPay）。辞退されたら返金する。
func TestReviewConsultationPayPerUseWithoutTickets(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupTicketSalesClosed(t)
	tadmin := adminToken(t, h)
	_, coach := onDutyCoach(t, h, tadmin)
	_, owner := demoRequester(t, h, tadmin)
	root := paidReview(t, h, owner, "paypay", "pi_paypay_review")
	offers := coachRequests(t, h, coach).Offers
	if len(offers) != 1 {
		t.Fatalf("offers %+v", offers)
	}
	if r := doJSON(t, h, "POST", "/v1/coach/offers/"+offers[0].ID+"/accept", coach, nil); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if r := doJSON(t, h, "POST", "/v1/coaching/requests/"+root+"/response", coach, responseBody()); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	path := "/v1/coaching/requests/" + root + "/consultations"
	room := doJSON(t, h, "GET", path, owner, nil)
	if room.Code != 200 || !strings.Contains(room.Body.String(), `"funding":"cash"`) || !strings.Contains(room.Body.String(), `"can_purchase":true`) {
		t.Fatalf("room %s", room.Body.String())
	}

	c := consultationDraft(t, h, owner, path, uuid.NewString(), "次の大会ではどこを意識しますか")
	if c.Funding != "cash" || c.Price != 600 {
		t.Fatalf("draft %+v", c)
	}
	rec := doJSON(t, h, "POST", "/v1/coaching/consultations/"+c.ID+"/submit", owner, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "checkout_url") {
		t.Fatalf("submit %d %s", rec.Code, rec.Body.String())
	}
	form := f.lastSession()
	if form.Get("payment_method_types[0]") != "card" || form.Get("payment_method_types[1]") != "paypay" || form.Get("payment_method_types[2]") != "" ||
		form.Get("line_items[0][price_data][unit_amount]") != "600" || form.Get("success_url") != "https://web.test/coaching/requests/"+root+"?consultation="+c.ID {
		t.Fatalf("session %v", form)
	}
	// 支払いが届くまでコーチには届かない。届いた知らせが2回来ても1回分。
	var status string
	queryRowArgs(t, `SELECT status FROM coaching_consultations WHERE id=$1`, []any{c.ID}, &status)
	if status != "awaiting_payment" {
		t.Fatal(status)
	}
	for i := 0; i < 2; i++ {
		if code := payConsultation(t, h, c.ID, 600); code != 200 {
			t.Fatalf("pay %d", code)
		}
	}
	consultationAction(t, h, coach, c.ID, "accept", nil)
	consultationAction(t, h, coach, c.ID, "answer", map[string]any{"body": "前半の接地を意識してください"})
	var count, earnings int
	queryRowArgs(t, `SELECT count(*),sum(amount_yen)::integer FROM coach_earnings WHERE consultation_id=$1`, []any{c.ID}, &count, &earnings)
	if count != 1 || earnings != 420 {
		t.Fatalf("earnings %d %d", count, earnings)
	}

	// 2件目は辞退されたので、カードでも PayPay でも同じ返金 API で全額を返す。
	c2 := consultationDraft(t, h, owner, path, uuid.NewString(), "練習の量を相談したいです")
	consultationAction(t, h, owner, c2.ID, "submit", nil)
	if code := payConsultation(t, h, c2.ID, 600); code != 200 {
		t.Fatalf("pay %d", code)
	}
	consultationAction(t, h, coach, c2.ID, "decline", nil)
	for i := 0; i < 2; i++ {
		if r := tick(t, h, "tick-secret"); r.Code != 200 {
			t.Fatal(r.Body.String())
		}
	}
	refunds := f.refundList()
	if len(refunds) != 1 || refunds[0].Get("amount") != "600" || refunds[0].Get("payment_intent") != "pi_consult_"+c2.ID {
		t.Fatalf("refunds %+v", refunds)
	}
	var state string
	queryRowArgs(t, `SELECT payment_state FROM coaching_consultations WHERE id=$1`, []any{c2.ID}, &state)
	if state != "refunded" {
		t.Fatal(state)
	}
}
