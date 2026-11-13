package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

func setupRefundNoticeTest(t *testing.T) (http.Handler, *fakeStripe, string, *atomic.Int32) {
	t.Helper()
	var pushes atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		pushes.Add(1)
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(server.Close)
	private, public, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	h, f := setupStripeConfig(t, func(cfg *config.Config) {
		cfg.VAPIDPublicKey = public
		cfg.VAPIDPrivateKey = private
		cfg.VAPIDSubject = "mailto:test@example.com"
		cfg.PushHTTP = server.Client()
	})
	return h, f, server.URL, &pushes
}

func subscribeRefundNoticeTest(t *testing.T, h http.Handler, payer, endpoint string) {
	t.Helper()
	p256dh, auth := testPushKeys(t)
	if rec := doJSON(t, h, "POST", "/v1/me/push-subscriptions", payer, map[string]any{
		"endpoint": endpoint, "keys": map[string]string{"p256dh": p256dh, "auth": auth},
	}); rec.Code != http.StatusNoContent {
		t.Fatalf("subscribe %d %s", rec.Code, rec.Body.String())
	}
	for _, category := range []string{"review", "coaching"} {
		if rec := doJSON(t, h, "PUT", "/v1/me/notification-settings/"+category, payer, map[string]bool{
			"in_app": false, "push": false,
		}); rec.Code != http.StatusOK {
			t.Fatalf("settings %d %s", rec.Code, rec.Body.String())
		}
	}
}

func assertRefundNotice(t *testing.T, h http.Handler, payer, title, url string, pushes *atomic.Int32, want int) {
	t.Helper()
	notes, _ := listNotifications(t, h, payer)
	count := 0
	for _, note := range notes {
		if note.Title != title {
			continue
		}
		if note.URL != url {
			t.Errorf("notice URL %q, want %q", note.URL, url)
		}
		count++
	}
	if count != want || int(pushes.Load()) != want {
		t.Fatalf("bell count = %d, push count = %d, want %d", count, pushes.Load(), want)
	}
}

func tickRefundNoticeTest(t *testing.T, h http.Handler) {
	t.Helper()
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
}

func TestReviewAuthorizationCancellationCompletionNotice(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f, endpoint, pushes := setupRefundNoticeTest(t)
	_, payer := demoRequester(t, h, adminToken(t, h))
	id := paidReview(t, h, payer, "card", "pi_card_notice")
	subscribeRefundNoticeTest(t, h, payer, endpoint)
	// 期限切れ通知のベルは受ける（プッシュは止めたまま）。
	if rec := doJSON(t, h, "PUT", "/v1/me/notification-settings/review", payer, map[string]bool{
		"in_app": true, "push": false,
	}); rec.Code != http.StatusOK {
		t.Fatalf("settings %d %s", rec.Code, rec.Body.String())
	}
	title, url := "フォーム添削の仮押さえの取り消しが完了しました", "/coaching/requests/"+id
	tickRefundNoticeTest(t, h)
	assertRefundNotice(t, h, payer, title, url, pushes, 0)

	execSQLArgs(t, "UPDATE review_requests SET due_at = now()-interval '1 minute' WHERE id=$1", id)
	execSQLArgs(t, `UPDATE review_payments SET authorized_at=now()-interval '2 minutes' WHERE request_id=$1`, id)
	tickRefundNoticeTest(t, h)
	if got := reviewPaymentStatus(t, id); got != "canceled" {
		t.Fatalf("cancellation did not commit: %s", got)
	}
	// 期限切れ系は1通に統合（本体 #573）：取り消しの別通知は届かず、期限切れ通知だけが届く。
	assertRefundNotice(t, h, payer, title, url, pushes, 0)
	var expired int
	queryRowArgs(t, "SELECT count(*) FROM notifications WHERE review_request_id=$1 AND kind='review_expired'", []any{id}, &expired)
	if expired != 1 {
		t.Fatalf("review_expired bells = %d, want 1", expired)
	}
	if _, canceled := f.calls(); len(canceled) != 1 {
		t.Fatalf("cancellation calls %d", len(canceled))
	}
}

func TestCapturedRefundCompletionNotice(t *testing.T) {
	for _, tc := range []struct {
		name               string
		admin, synchronous bool
	}{
		{name: "admin_immediate_success", admin: true, synchronous: true},
		{name: "admin_then_webhook", admin: true},
		{name: "stripe_dashboard"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakeCoachingCloudinary(t)
			h, f, endpoint, pushes := setupRefundNoticeTest(t)
			admin := adminToken(t, h)
			_, payer := demoRequester(t, h, admin)
			intent := "pi_card_captured_notice"
			id := paidReview(t, h, payer, "card", intent)
			legacyReviewAmount(t, id)
			execSQLArgs(t, "UPDATE review_payments SET status='captured',captured_at=now() WHERE request_id=$1", id)
			subscribeRefundNoticeTest(t, h, payer, endpoint)
			title, url := "フォーム添削の返金が完了しました", "/coaching/requests/"+id
			remoteID := "re_dashboard_notice"
			event := func(status string, amount int) {
				t.Helper()
				rec := stripeWebhook(t, h, testWebhookSecret, "refund.updated", map[string]any{
					"id": remoteID, "currency": "jpy", "payment_intent": intent, "amount": amount, "status": status,
				})
				if rec.Code != http.StatusOK {
					t.Fatalf("webhook %d %s", rec.Code, rec.Body.String())
				}
			}
			assertRefundNotice(t, h, payer, title, url, pushes, 0)
			if tc.admin {
				var payment string
				queryRowArgs(t, "SELECT id::text FROM review_payments WHERE request_id=$1", []any{id}, &payment)
				if !tc.synchronous {
					f.mu.Lock()
					f.ticketRefundStatus = "pending"
					f.mu.Unlock()
				}
				x := capturedFixture{h: h, admin: admin, payment: payment}
				x.refund(t)
				remoteID = "re_1"
			} else {
				event("succeeded", 500)
				assertRefundNotice(t, h, payer, title, url, pushes, 0)
				event("pending", 1500)
			}
			wantState, wantNotices := "refund_pending", 0
			if tc.synchronous {
				wantState, wantNotices = "refunded", 1
			}
			if got := reviewPaymentStatus(t, id); got != wantState {
				t.Fatalf("payment state %s, want %s", got, wantState)
			}
			assertRefundNotice(t, h, payer, title, url, pushes, wantNotices)
			for range 2 {
				event("succeeded", 1500)
				assertRefundNotice(t, h, payer, title, url, pushes, 1)
			}
			event("pending", 1500)
			assertRefundNotice(t, h, payer, title, url, pushes, 1)
		})
	}
}

func TestConsultationExpiryRefundCompletionNotice(t *testing.T) {
	h, f, endpoint, pushes := setupRefundNoticeTest(t)
	owner, coach, root := endedCashRoom(t, h)
	c := consultationDraft(t, h, owner, "/v1/coaching/contracts/"+root+"/consultations", uuid.NewString(), "返金通知の確認")
	consultationAction(t, h, owner, c.ID, "submit", nil)
	if code := payConsultation(t, h, c.ID, 1200); code != 200 {
		t.Fatalf("payment webhook %d", code)
	}
	consultationAction(t, h, coach, c.ID, "accept", nil)
	subscribeRefundNoticeTest(t, h, owner, endpoint)
	title, url := "追加相談の返金が完了しました", "/coaching/personal/"+root
	execSQLArgs(t, "UPDATE coaching_consultations SET answer_by=now()-interval '1 minute' WHERE id=$1", c.ID)
	f.mu.Lock()
	f.ticketRefundStatus = "pending"
	f.mu.Unlock()
	tickRefundNoticeTest(t, h)
	var state string
	queryRowArgs(t, "SELECT payment_state FROM coaching_consultations WHERE id=$1", []any{c.ID}, &state)
	if state != "refund_pending" {
		t.Fatalf("pending payment %s", state)
	}
	assertRefundNotice(t, h, owner, title, url, pushes, 0)
	f.mu.Lock()
	f.ticketRefundStatus = "succeeded"
	f.mu.Unlock()
	for range 2 {
		tickRefundNoticeTest(t, h)
		assertRefundNotice(t, h, owner, title, url, pushes, 1)
	}
}

func TestReviewQuestionExpiryRefundCompletionNotice(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f, endpoint, pushes := setupRefundNoticeTest(t)
	admin := adminToken(t, h)
	_, coach := approvedReviewCoach(t, h, admin)
	p256dh, auth := testPushKeys(t)
	if rec := doJSON(t, h, "POST", "/v1/me/push-subscriptions", coach, map[string]any{
		"endpoint": endpoint, "keys": map[string]string{"p256dh": p256dh, "auth": auth},
	}); rec.Code != http.StatusNoContent {
		t.Fatalf("coach subscribe %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", coach, map[string]any{"on_duty": true}); rec.Code != http.StatusOK {
		t.Fatalf("on duty %d %s", rec.Code, rec.Body.String())
	}
	// 受付開始の接続テストが済んだらコーチの購読は外す。以後のプッシュ計数は依頼者だけにする。
	if rec := doJSON(t, h, http.MethodDelete, "/v1/me/push-subscriptions", coach, map[string]any{"endpoint": endpoint}); rec.Code != http.StatusNoContent {
		t.Fatalf("coach unsubscribe %d %s", rec.Code, rec.Body.String())
	}
	pushes.Store(0)
	_, owner := demoRequester(t, h, admin)
	root := paidReview(t, h, owner, "card", "pi_card_question_notice")
	offers := coachRequests(t, h, coach).Reviewing
	if len(offers) != 1 {
		t.Fatalf("offers %+v", offers)
	}
	if r := doJSON(t, h, "POST", "/v1/coach/requests/"+offers[0].ID+"/start", coach, nil); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if r := doJSON(t, h, "POST", "/v1/coaching/requests/"+root+"/response", coach, responseBody()); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	c := consultationDraft(t, h, owner, "/v1/coaching/requests/"+root+"/consultations", uuid.NewString(), "追加質問の返金通知")
	consultationAction(t, h, owner, c.ID, "submit", nil)
	if code := payConsultation(t, h, c.ID, 500); code != 200 {
		t.Fatalf("payment webhook %d", code)
	}
	consultationAction(t, h, coach, c.ID, "accept", nil)
	subscribeRefundNoticeTest(t, h, owner, endpoint)
	title, url := "追加質問の返金が完了しました", "/coaching/requests/"+root
	execSQLArgs(t, "UPDATE coaching_consultations SET answer_by=now()-interval '1 minute' WHERE id=$1", c.ID)
	f.mu.Lock()
	f.ticketRefundStatus = "pending"
	f.mu.Unlock()
	tickRefundNoticeTest(t, h)
	var state string
	queryRowArgs(t, "SELECT payment_state FROM coaching_consultations WHERE id=$1", []any{c.ID}, &state)
	if state != "refund_pending" {
		t.Fatalf("pending payment %s", state)
	}
	assertRefundNotice(t, h, owner, title, url, pushes, 0)
	f.mu.Lock()
	f.ticketRefundStatus = "succeeded"
	f.mu.Unlock()
	for range 2 {
		tickRefundNoticeTest(t, h)
		assertRefundNotice(t, h, owner, title, url, pushes, 1)
	}
}

func TestPendingReviewPayPayRefundCompletionNotice(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f, endpoint, pushes := setupRefundNoticeTest(t)
	_, payer := demoRequester(t, h, adminToken(t, h))
	intent := "pi_paypay_notice"
	id := paidReview(t, h, payer, "paypay", intent)
	legacyReviewAmount(t, id)
	subscribeRefundNoticeTest(t, h, payer, endpoint)
	title, url := "フォーム添削の返金が完了しました", "/coaching/requests/"+id
	f.mu.Lock()
	f.ticketRefundStatus = "pending"
	f.mu.Unlock()
	execSQLArgs(t, "UPDATE review_requests SET due_at=now()-interval '1 minute' WHERE id=$1", id)
	execSQLArgs(t, "UPDATE review_payments SET authorized_at=now()-interval '2 minutes' WHERE request_id=$1", id)
	tickRefundNoticeTest(t, h)
	assertRefundNotice(t, h, payer, title, url, pushes, 0)
	for _, amount := range []int{500, 1500, 1500} {
		rec := stripeWebhook(t, h, testWebhookSecret, "refund.updated", map[string]any{
			"id": "re_1", "currency": "jpy", "payment_intent": intent, "amount": amount, "status": "succeeded",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("webhook %d %s", rec.Code, rec.Body.String())
		}
		if amount == 500 {
			assertRefundNotice(t, h, payer, title, url, pushes, 0)
		} else {
			assertRefundNotice(t, h, payer, title, url, pushes, 1)
		}
	}
	if got := len(f.refundList()); got != 1 {
		t.Fatalf("Stripe refund calls %d", got)
	}
}
