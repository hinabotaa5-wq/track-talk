package httpx_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// 始めずに返金すると決めた支払い（must_refund）の受け入れテスト。
// 非JPカードで止めた支払いは、返金が Stripe で失敗しても、契約がほかの支払いで始まっていても、
// tick が返金し終えるまで拾い続ける。その支払いは、二重払いの判定・報酬・精算で契約の代金として数えない。
// 返金待ちの支払いがある契約では、返金し終えるまで支払い画面を作り直さない。

// paymentBySession は、支払い画面ごとの支払いの状態。
func paymentBySession(t *testing.T, session string) (status string, mustRefund bool, refundedYen int, reason string) {
	t.Helper()
	queryRowArgs(t, `SELECT status, must_refund, refunded_yen, coalesce(refund_reason, '') FROM coaching_payments WHERE checkout_session_id = $1`,
		[]any{session}, &status, &mustRefund, &refundedYen, &reason)
	return
}

// refundsFor は、偽 Stripe に送られた返金のうち、その PaymentIntent への金額の一覧。
func refundsFor(f *fakeStripe, intent string) []string {
	var out []string
	for _, r := range f.refundList() {
		if r.Get("payment_intent") == intent {
			out = append(out, r.Get("amount"))
		}
	}
	return out
}

// failRefundsFor は、その PaymentIntent への全額返金を Stripe 側の失敗にする（返金できる上限を1円にする）。
func failRefundsFor(f *fakeStripe, intent string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.chargeAmounts = map[string]int{intent: 1}
}

// forgetRefundKeys は、失敗した返金の Idempotency-Key の記録を消す。一時的な失敗のあとの送り直しとして、
// 次の返金も偽 Stripe に実際に処理させる（記録が残っていると、偽 Stripe は前の返金を成功として返す）。
func forgetRefundKeys(f *fakeStripe) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refundKeys = nil
}

// allowRefunds は、返金が通るように戻す。
func allowRefunds(f *fakeStripe) {
	forgetRefundKeys(f)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.chargeAmounts = nil
}

// agePayments は、tick の拾い直しの対象（払ってから1分以上）になるよう、契約の支払いの時刻を同じだけ過去へずらす（順序は保つ）。
func agePayments(t *testing.T, contractID string) {
	t.Helper()
	execSQLArgs(t, `UPDATE coaching_payments SET paid_at = paid_at - interval '5 minutes' WHERE contract_id = $1 AND paid_at IS NOT NULL`, contractID)
}

// contractPayments は、返金・報酬・精算が契約の代金として読む支払い。
func contractPayments(t *testing.T, contractID string) (db.CoachingPayment, db.GetSupportPaidRow) {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	q := db.New(pool)
	id := uuid.MustParse(contractID)
	paid, err := q.GetPaidCoachingPayment(ctx, &id)
	if err != nil {
		t.Fatalf("paid payment %v", err)
	}
	support, err := q.GetSupportPaid(ctx, &id)
	if err != nil {
		t.Fatalf("support paid %v", err)
	}
	return paid, support
}

// TestContractPaymentNonJPRefundFailureRefundedAfterDomesticStart は細目1：非JPの返金が Stripe で失敗したまま、
// 生きていた別の支払い画面から国内カードで払われて契約が始まっても、tick は非JPの支払いを全額返金し、
// 国内カードの支払いは返金しない。契約は始まったまま。
func TestContractPaymentNonJPRefundFailureRefundedAfterDomesticStart(t *testing.T) {
	h, f := setupStripe(t)
	tu, _, id := adultAccepted(t, h)
	checkout(t, h, tu, id)
	s1 := sessionIDFor(t, id)
	// 1つ目の画面の期限が近いと、次の申し込みでは新しい画面を作る（再利用は残り5分超のときだけ）。2つの画面が同時に生きている。
	execSQLArgs(t, `UPDATE coaching_payments SET session_expires_at = now() + interval '3 minutes' WHERE checkout_session_id = $1`, s1)
	checkout(t, h, tu, id)
	s2 := sessionIDFor(t, id)
	if s1 == s2 {
		t.Fatal("expected a second checkout session")
	}

	failRefundsFor(f, "pi_nonjp_mr_1")
	if rec := completeContractWebhook(t, h, s1, "pi_nonjp_mr_1"); rec.Code != http.StatusOK {
		t.Fatalf("nonjp webhook %d %s", rec.Code, rec.Body.String())
	}
	if st, must, _, _ := paymentBySession(t, s1); st != "paid" || !must || contractStatus(t, id) != "accepted" {
		t.Fatalf("after failed refund: payment %s must_refund %v contract %s", st, must, contractStatus(t, id))
	}
	allowRefunds(f)
	if rec := completeContractWebhook(t, h, s2, "pi_jp_mr_2"); rec.Code != http.StatusOK || contractStatus(t, id) != "active" {
		t.Fatalf("jp webhook %d contract %s", rec.Code, contractStatus(t, id))
	}
	// 返金し終える前でも、契約の代金として読むのは国内カードの支払い。
	if paid, support := contractPayments(t, id); paid.PaymentIntentID == nil || *paid.PaymentIntentID != "pi_jp_mr_2" || support.RefundedYen != 0 || support.AmountYen != 8000 {
		t.Fatalf("contract payment before tick %+v %+v", paid, support)
	}

	agePayments(t, id)
	for i := 0; i < 2; i++ {
		if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK {
			t.Fatalf("tick %d", rec.Code)
		}
	}
	if st, _, yen, reason := paymentBySession(t, s1); st != "refunded" || yen != 8000 || reason != "not_started" {
		t.Fatalf("nonjp payment %s %d %s", st, yen, reason)
	}
	if got := refundsFor(f, "pi_nonjp_mr_1"); len(got) != 1 || got[0] != "8000" {
		t.Fatalf("nonjp refunds %v", got)
	}
	if st, must, yen, _ := paymentBySession(t, s2); st != "paid" || must || yen != 0 {
		t.Fatalf("jp payment %s %v %d", st, must, yen)
	}
	if got := refundsFor(f, "pi_jp_mr_2"); len(got) != 0 {
		t.Fatalf("jp payment refunded %v", got)
	}
	if contractStatus(t, id) != "active" || !contractStarted(t, id) {
		t.Fatalf("contract %s", contractStatus(t, id))
	}
	if paid, support := contractPayments(t, id); *paid.PaymentIntentID != "pi_jp_mr_2" || support.RefundedYen != 0 || support.AmountYen != 8000 {
		t.Fatalf("contract payment after tick %+v %+v", paid, support)
	}
}

// TestContractPaymentNonJPRefundFailureBlocksNewCheckout は、非JPの返金が Stripe で失敗しているあいだ、
// 同じ契約の支払い画面を作り直させない。tick が返金し終えたら、国内カードで払い直せて、その代金は返金されない。
func TestContractPaymentNonJPRefundFailureBlocksNewCheckout(t *testing.T) {
	h, f := setupStripe(t)
	tu, _, id := adultAccepted(t, h)
	checkout(t, h, tu, id)
	s1 := sessionIDFor(t, id)

	failRefundsFor(f, "pi_nonjp_mr_3")
	if rec := completeContractWebhook(t, h, s1, "pi_nonjp_mr_3"); rec.Code != http.StatusOK {
		t.Fatalf("nonjp webhook %d %s", rec.Code, rec.Body.String())
	}
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/checkout", tu, nil)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "refund_pending") {
		t.Fatalf("checkout while refund pending %d %s", rec.Code, rec.Body.String())
	}
	if sessionIDFor(t, id) != s1 {
		t.Fatal("a new checkout session was created while a refund is pending")
	}

	// 返金が失敗し続けるあいだは、支払いは返金待ちのまま残り、支払い画面も作り直さない。
	agePayments(t, id)
	forgetRefundKeys(f)
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatalf("tick %d", rec.Code)
	}
	if st, must, _, _ := paymentBySession(t, s1); st != "paid" || !must {
		t.Fatalf("payment while refund fails %s %v", st, must)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/checkout", tu, nil); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("checkout while refund still pending %d", rec.Code)
	}

	allowRefunds(f)
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatalf("tick %d", rec.Code)
	}
	if st, _, yen, reason := paymentBySession(t, s1); st != "refunded" || yen != 8000 || reason != "not_started" {
		t.Fatalf("nonjp payment %s %d %s", st, yen, reason)
	}
	if got := refundsFor(f, "pi_nonjp_mr_3"); len(got) != 1 || got[0] != "8000" {
		t.Fatalf("nonjp refunds %v", got)
	}
	if contractStatus(t, id) != "accepted" || contractStarted(t, id) {
		t.Fatalf("contract %s", contractStatus(t, id))
	}

	// 返金し終えたら、国内カードで払い直せる。その支払いは二重払いとして返金されない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/checkout", tu, nil); rec.Code != http.StatusOK {
		t.Fatalf("checkout after refund %d %s", rec.Code, rec.Body.String())
	}
	s2 := sessionIDFor(t, id)
	if s2 == s1 {
		t.Fatal("expected a new checkout session after the refund")
	}
	if rec := completeContractWebhook(t, h, s2, "pi_jp_mr_4"); rec.Code != http.StatusOK || contractStatus(t, id) != "active" {
		t.Fatalf("jp webhook %d contract %s", rec.Code, contractStatus(t, id))
	}
	agePayments(t, id)
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatalf("tick %d", rec.Code)
	}
	if got := refundsFor(f, "pi_jp_mr_4"); len(got) != 0 {
		t.Fatalf("jp payment refunded %v", got)
	}
	if st, _, _, _ := paymentBySession(t, s2); st != "paid" || contractStatus(t, id) != "active" {
		t.Fatalf("jp payment %s contract %s", st, contractStatus(t, id))
	}
}

// TestContractPaymentNonJPRefundedThenDomesticRepayKeepsPayment は、非JPで止めて返金し終えた契約に国内カードで払い直したとき、
// tick が国内カードの支払いを二重払いとして返金しない。契約は始まったまま、報酬・精算は国内カードの支払いで数える。
func TestContractPaymentNonJPRefundedThenDomesticRepayKeepsPayment(t *testing.T) {
	h, f := setupStripe(t)
	tu, _, id := adultAccepted(t, h)
	checkout(t, h, tu, id)
	s1 := sessionIDFor(t, id)
	if rec := completeContractWebhook(t, h, s1, "pi_nonjp_mr_5"); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("nonjp webhook %d %s", rec.Code, rec.Body.String())
	}
	if st, must, yen, _ := paymentBySession(t, s1); st != "refunded" || !must || yen != 8000 {
		t.Fatalf("nonjp payment %s %v %d", st, must, yen)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/checkout", tu, nil); rec.Code != http.StatusOK {
		t.Fatalf("checkout after refund %d %s", rec.Code, rec.Body.String())
	}
	s2 := sessionIDFor(t, id)
	if rec := completeContractWebhook(t, h, s2, "pi_jp_mr_6"); rec.Code != http.StatusOK || contractStatus(t, id) != "active" {
		t.Fatalf("jp webhook %d contract %s", rec.Code, contractStatus(t, id))
	}
	agePayments(t, id)
	for i := 0; i < 2; i++ {
		if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK {
			t.Fatalf("tick %d", rec.Code)
		}
	}
	if got := refundsFor(f, "pi_jp_mr_6"); len(got) != 0 {
		t.Fatalf("jp payment refunded %v", got)
	}
	if got := refundsFor(f, "pi_nonjp_mr_5"); len(got) != 1 {
		t.Fatalf("nonjp refunds %v", got)
	}
	if st, must, _, _ := paymentBySession(t, s2); st != "paid" || must || contractStatus(t, id) != "active" {
		t.Fatalf("jp payment %s %v contract %s", st, must, contractStatus(t, id))
	}
	if paid, support := contractPayments(t, id); *paid.PaymentIntentID != "pi_jp_mr_6" || support.RefundedYen != 0 || support.AmountYen != 8000 {
		t.Fatalf("contract payment %+v %+v", paid, support)
	}
}
