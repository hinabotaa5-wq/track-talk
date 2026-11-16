package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 継続プランの契約決済にもカード発行国の確認を入れた分の受け入れテスト。
// 非JPカードでは契約を始めず、既存の未開始返金で全額返す。発行国が取れないときは通す。
// 偽 Stripe は pi_nonjp で始まる支払いを米国発行、pi_unknown で始まる支払いを発行国不明、
// pi_failread で始まる支払いを読み取り失敗として答える。

// completeContractWebhook は、指定した PaymentIntent で継続プランの契約決済の完了を知らせる。
func completeContractWebhook(t *testing.T, h http.Handler, session, intent string) *httptest.ResponseRecorder {
	t.Helper()
	return stripeWebhook(t, h, testWebhookSecret, "checkout.session.completed", map[string]any{
		"id": session, "payment_status": "paid", "payment_intent": intent, "amount_total": 8000,
	})
}

// contractStarted は、契約の期間が始まっているか。
func contractStarted(t *testing.T, id string) bool {
	t.Helper()
	var started bool
	queryRowArgs(t, `SELECT starts_at IS NOT NULL FROM coaching_contracts WHERE id = $1`, []any{id}, &started)
	return started
}

// coachingPaymentStatus は、契約の最新の支払いの状態を返す。
func coachingPaymentStatus(t *testing.T, contractID string) string {
	t.Helper()
	var status string
	queryRowArgs(t, `SELECT status FROM coaching_payments WHERE contract_id = $1 ORDER BY created_at DESC LIMIT 1`, []any{contractID}, &status)
	return status
}

// noticeCount は、契約の詳細画面にひもづく見出しの知らせの件数を返す。
func noticeCount(t *testing.T, contractID, title string) int {
	t.Helper()
	var n int
	queryRowArgs(t, `SELECT count(*) FROM notifications WHERE url = $1 AND title = $2`,
		[]any{"/coaching/personal/" + contractID, title}, &n)
	return n
}

// TestContractPaymentNonJPCardBlocked は細目1：非JPカードでは 422 で理由を返し、契約は始まらない。
func TestContractPaymentNonJPCardBlocked(t *testing.T) {
	h, _ := setupStripe(t)
	tu, _, id := adultAccepted(t, h)
	checkout(t, h, tu, id)
	session := sessionIDFor(t, id)

	rec := completeContractWebhook(t, h, session, "pi_nonjp_block_1")
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "card_country_not_supported") ||
		!strings.Contains(rec.Body.String(), "海外で発行されたカードはご利用いただけません") {
		t.Fatalf("webhook %d %s", rec.Code, rec.Body.String())
	}
	if contractStatus(t, id) != "accepted" || contractStarted(t, id) {
		t.Fatalf("started %s %v", contractStatus(t, id), contractStarted(t, id))
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/coaching/contracts/"+id, tu, nil)
	if ct := decodeContract(t, rec.Body.Bytes()); ct.Status != "accepted" || ct.StartsAt != nil {
		t.Fatalf("detail started %+v", ct)
	}
}

// TestContractPaymentNonJPCardRefunded は細目2：非JPカードでは全額返金され、開始の知らせは出ない。
func TestContractPaymentNonJPCardRefunded(t *testing.T) {
	h, f := setupStripe(t)
	tu, _, id := adultAccepted(t, h)
	checkout(t, h, tu, id)
	session := sessionIDFor(t, id)

	rec := completeContractWebhook(t, h, session, "pi_nonjp_refund_1")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("webhook %d %s", rec.Code, rec.Body.String())
	}
	refunds := f.refundList()
	if len(refunds) != 1 || refunds[0].Get("payment_intent") != "pi_nonjp_refund_1" || refunds[0].Get("amount") != "8000" {
		t.Fatalf("refunds %v", refunds)
	}
	if got := coachingPaymentStatus(t, id); got != "refunded" {
		t.Fatalf("payment status %q, want refunded", got)
	}
	if n := noticeCount(t, id, "パーソナル指導が始まりました"); n != 0 {
		t.Fatalf("start notice sent %d", n)
	}
	if n := noticeCount(t, id, "お支払いを返金しました"); n == 0 {
		t.Fatal("refund notice not sent")
	}
}

// TestContractPaymentJPCardStarts は細目3：JP カードでは従来どおり契約が始まり、返金はない。
func TestContractPaymentJPCardStarts(t *testing.T) {
	h, f := setupStripe(t)
	tu, _, id := adultAccepted(t, h)
	checkout(t, h, tu, id)

	paidSession(t, h, sessionIDFor(t, id), "pi_jp_ok_1", 8000)
	if contractStatus(t, id) != "active" || !contractStarted(t, id) {
		t.Fatalf("not started %s", contractStatus(t, id))
	}
	if len(f.refundList()) != 0 {
		t.Fatalf("refunds %v", f.refundList())
	}
}

// TestContractPaymentUnknownCountryStarts は細目4：発行国が取れない支払いでは従来どおり始まる。
func TestContractPaymentUnknownCountryStarts(t *testing.T) {
	h, f := setupStripe(t)
	tu, _, id := adultAccepted(t, h)
	checkout(t, h, tu, id)

	paidSession(t, h, sessionIDFor(t, id), "pi_unknown_ok_1", 8000)
	if contractStatus(t, id) != "active" || !contractStarted(t, id) {
		t.Fatalf("not started %s", contractStatus(t, id))
	}
	if len(f.refundList()) != 0 {
		t.Fatalf("refunds %v", f.refundList())
	}
}

// TestContractPaymentNonJPIdempotent は細目5：非JPの知らせの再送では返金は増えず、契約は始まらない。
// 開始済みの契約への2件目の支払いが非JPでも、返金は1件だけで契約は active のまま変わらない。
func TestContractPaymentNonJPIdempotent(t *testing.T) {
	h, f := setupStripe(t)
	tu, _, id := adultAccepted(t, h)
	checkout(t, h, tu, id)
	session := sessionIDFor(t, id)

	if rec := completeContractWebhook(t, h, session, "pi_nonjp_idem_1"); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("block %d %s", rec.Code, rec.Body.String())
	}
	if rec := completeContractWebhook(t, h, session, "pi_nonjp_idem_1"); rec.Code != http.StatusOK {
		t.Fatalf("redeliver %d %s", rec.Code, rec.Body.String())
	}
	refunds := f.refundList()
	if len(refunds) != 1 || refunds[0].Get("payment_intent") != "pi_nonjp_idem_1" {
		t.Fatalf("double refund %v", refunds)
	}
	if contractStatus(t, id) != "accepted" || contractStarted(t, id) {
		t.Fatal("redelivery started the contract")
	}

	// 開始済みの契約への2件目の支払い（非JP）は全額返金だけする（別契約で確かめる）。
	tu2, _, id2 := adultAccepted(t, h)
	checkout(t, h, tu2, id2)
	paidSession(t, h, sessionIDFor(t, id2), "pi_jp_first_1", 8000)
	if contractStatus(t, id2) != "active" {
		t.Fatal("first payment did not start the contract")
	}
	execSQLArgs(t, `INSERT INTO coaching_payments (id, contract_id, payer, amount_yen, checkout_session_id, checkout_url, session_expires_at)
		VALUES (gen_random_uuid(), $1, 'requester', 8000, 'cs_dup_nonjp', 'https://checkout.stripe.test/dup', now() + interval '1 hour')`, id2)
	if rec := completeContractWebhook(t, h, "cs_dup_nonjp", "pi_nonjp_dup_1"); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("duplicate block %d %s", rec.Code, rec.Body.String())
	}
	var dup int
	queryRowArgs(t, `SELECT count(*) FROM coaching_payments WHERE contract_id = $1 AND payment_intent_id = 'pi_nonjp_dup_1' AND status = 'refunded'`, []any{id2}, &dup)
	if dup != 1 {
		t.Fatalf("duplicate not refunded once %d", dup)
	}
	if contractStatus(t, id2) != "active" {
		t.Fatal("duplicate payment changed the contract")
	}
}

// TestContractPaymentNonJPCardByGuardian は細目6：保護者が払う場合も非JPカードでは始めず全額返す。
func TestContractPaymentNonJPCardByGuardian(t *testing.T) {
	h, f := setupStripe(t)
	tm, gt := guardianMinor(t, h)
	_, id := minorAccepted(t, h, tm)
	gpath := "/v1/guardian/" + gt + "/contracts/" + id

	if rec := doJSON(t, h, http.MethodPost, gpath+"/consent", "", map[string]any{"agree_report": true, "agree_detection": true}); rec.Code != 422 || !strings.Contains(rec.Body.String(), "monthly_limit") {
		t.Fatalf("guardian monthly limit %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPatch, "/v1/guardian/"+gt, "", map[string]any{"monthly_limit_yen": nil, "allow_publish": false}); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, gpath+"/consent", "", map[string]any{"agree_report": true, "agree_detection": true}); rec.Code != http.StatusOK {
		t.Fatalf("guardian consent %d %s", rec.Code, rec.Body.String())
	}
	if rec := completeContractWebhook(t, h, sessionIDFor(t, id), "pi_nonjp_guardian_1"); rec.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(rec.Body.String(), "card_country_not_supported") {
		t.Fatalf("webhook %d %s", rec.Code, rec.Body.String())
	}
	if contractStatus(t, id) != "accepted" || contractStarted(t, id) {
		t.Fatal("guardian non-JP payment started the contract")
	}
	refunds := f.refundList()
	if len(refunds) != 1 || refunds[0].Get("payment_intent") != "pi_nonjp_guardian_1" || refunds[0].Get("amount") != "8000" {
		t.Fatalf("refunds %v", refunds)
	}
}

// TestContractPaymentReadFailureRetries は細目7：発行国の読み取りに失敗したら 500 にし、支払いは pending のまま残る。
func TestContractPaymentReadFailureRetries(t *testing.T) {
	h, f := setupStripe(t)
	tu, _, id := adultAccepted(t, h)
	checkout(t, h, tu, id)
	session := sessionIDFor(t, id)

	if rec := completeContractWebhook(t, h, session, "pi_failread_1"); rec.Code != http.StatusInternalServerError {
		t.Fatalf("read failure %d %s", rec.Code, rec.Body.String())
	}
	if contractStatus(t, id) != "accepted" || contractStarted(t, id) {
		t.Fatal("failed read started the contract")
	}
	if got := coachingPaymentStatus(t, id); got != "pending" {
		t.Fatalf("payment status %q, want pending", got)
	}
	if len(f.refundList()) != 0 {
		t.Fatalf("refunds %v", f.refundList())
	}
	// 再送（JP カードで読み直し）では従来どおり始まる。
	paidSession(t, h, session, "pi_jp_retry_1", 8000)
	if contractStatus(t, id) != "active" {
		t.Fatal("retry did not start the contract")
	}
}
