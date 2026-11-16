package httpx_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
)

// 決済完了まわりの堅牢化（カード発行国不明のログ、金額・通貨の照合、Webhook 鍵と出所の照合）のテスト。

// captureSlog は、テストの間だけ slog の出力を捕まえる。
func captureSlog(t *testing.T) *lockedBuffer {
	t.Helper()
	buf := &lockedBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return buf
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// 発行国が取れない支払いでは、通したうえで種類つきの構造化ログを出す。分かる支払いでは出さない。
func TestUnknownCardCountryIsLoggedWithoutBlocking(t *testing.T) {
	buf := captureSlog(t)
	h, _ := setupStripe(t)
	tu, _, id := adultAccepted(t, h)
	checkout(t, h, tu, id)
	paidSession(t, h, sessionIDFor(t, id), "pi_unknown_log_1", 8000)
	if contractStatus(t, id) != "active" {
		t.Fatalf("blocked %s", contractStatus(t, id))
	}
	out := buf.String()
	if !strings.Contains(out, "card_country_unknown") || !strings.Contains(out, "payment_kind=support") {
		t.Fatalf("no log: %s", out)
	}
	for _, bad := range []string{"pi_unknown_log_1", "pm_test_unknown"} {
		if strings.Contains(out, bad) {
			t.Fatalf("log leaks %s: %s", bad, out)
		}
	}

	buf2 := captureSlog(t)
	tu2, _, id2 := adultAccepted(t, h)
	checkout(t, h, tu2, id2)
	paidSession(t, h, sessionIDFor(t, id2), "pi_known_log_1", 8000)
	if strings.Contains(buf2.String(), "card_country_unknown") {
		t.Fatalf("logged for known country: %s", buf2.String())
	}
}

// 1回プランの完了通知は、金額・通貨が支払いと合わなければ完了させない。
func TestReviewCheckoutAmountAndCurrencyMustMatch(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	_, _ = onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)
	id, session := createCardReview(t, h, tu)
	status := func() string {
		var s string
		queryRowArgs(t, `SELECT status FROM review_payments WHERE checkout_session_id=$1`, []any{session}, &s)
		return s
	}
	for name, obj := range map[string]map[string]any{
		"金額違い": {"amount_total": 1, "currency": "jpy"},
		"通貨違い": {"currency": "usd"},
	} {
		obj["id"], obj["payment_status"], obj["payment_intent"] = session, "unpaid", "pi_card_mismatch"
		obj["metadata"] = map[string]any{"kind": "review"}
		if rec := stripeWebhook(t, h, testWebhookSecret, "checkout.session.completed", obj); rec.Code != http.StatusInternalServerError {
			t.Fatalf("%s: webhook %d %s", name, rec.Code, rec.Body.String())
		}
		if status() != "pending" {
			t.Fatalf("%s: status %s", name, status())
		}
	}
	_ = id
	// 正しい金額・通貨なら従来どおり進む（金額は支払いの行から補われる）。
	if rec := completeCardWebhook(t, h, session, "pi_card_match"); rec.Code != http.StatusOK {
		t.Fatalf("ok webhook %d %s", rec.Code, rec.Body.String())
	}
	if status() == "pending" {
		t.Fatal("not completed")
	}
}

// 継続プランの契約決済は、契約の版にかかわらず金額・通貨が合わなければ始めない。
func TestContractCheckoutMismatchDoesNotStart(t *testing.T) {
	h, _ := setupStripe(t)
	tu, _, id := adultAccepted(t, h)
	checkout(t, h, tu, id)
	session := sessionIDFor(t, id)
	for name, obj := range map[string]map[string]any{
		"金額違い": {"amount_total": 1, "currency": "jpy"},
		"通貨違い": {"amount_total": 8000, "currency": "usd"},
	} {
		obj["id"], obj["payment_status"], obj["payment_intent"] = session, "paid", "pi_mismatch_"+name
		if rec := stripeWebhook(t, h, testWebhookSecret, "checkout.session.completed", obj); rec.Code != http.StatusInternalServerError {
			t.Fatalf("%s: webhook %d %s", name, rec.Code, rec.Body.String())
		}
		if contractStatus(t, id) != "accepted" || coachingPaymentStatus(t, id) != "pending" {
			t.Fatalf("%s: moved %s %s", name, contractStatus(t, id), coachingPaymentStatus(t, id))
		}
	}
	paidSession(t, h, session, "pi_ok_after_mismatch", 8000)
	if contractStatus(t, id) != "active" {
		t.Fatalf("not started %s", contractStatus(t, id))
	}
}

// Connect 用の鍵で届いたイベントは、口座の同期など許した種類だけ。決済の完了は処理しない。
func TestConnectKeyCannotCompleteCheckout(t *testing.T) {
	h, _ := setupStripe(t)
	tu, _, id := adultAccepted(t, h)
	checkout(t, h, tu, id)
	session := sessionIDFor(t, id)
	obj := map[string]any{"id": session, "payment_status": "paid", "payment_intent": "pi_origin_1", "amount_total": 8000, "currency": "jpy"}
	if rec := stripeWebhookAt(t, h, "/webhooks/stripe", testConnectWebhookSecret, "checkout.session.completed", obj); rec.Code != http.StatusBadRequest {
		t.Fatalf("connect key completed %d %s", rec.Code, rec.Body.String())
	}
	if contractStatus(t, id) != "accepted" {
		t.Fatalf("started by connect key: %s", contractStatus(t, id))
	}
	if rec := stripeWebhookAt(t, h, "/webhooks/stripe", testConnectWebhookSecret, "charge.dispute.created", map[string]any{"id": "dp_1"}); rec.Code != http.StatusBadRequest {
		t.Fatalf("connect key dispute %d", rec.Code)
	}
	if rec := stripeWebhookAt(t, h, "/webhooks/stripe", testConnectWebhookSecret, "account.updated", map[string]any{"id": "acct_unknown"}); rec.Code != http.StatusOK {
		t.Fatalf("connect key account.updated %d", rec.Code)
	}
	// 本体用の鍵では従来どおり完了する。
	paidSession(t, h, session, "pi_origin_2", 8000)
	if contractStatus(t, id) != "active" {
		t.Fatalf("main key did not start: %s", contractStatus(t, id))
	}
}
