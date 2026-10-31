package httpx_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"testing"

	"github.com/google/uuid"
)

type capturedFixture struct {
	h                        http.Handler
	f                        *fakeStripe
	admin, user              string
	coach                    uuid.UUID
	payment, request, intent string
}

func newCapturedFixture(t *testing.T) capturedFixture {
	t.Helper()
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	admin := adminToken(t, h)
	coach, _ := onDutyCoach(t, h, admin)
	_, user := demoRequester(t, h, admin)
	intent := "pi_captured_" + uuid.NewString()
	request := paidReview(t, h, user, "card", intent)
	execSQLArgs(t, `UPDATE review_requests SET coach_id=$2,status='returned',returned_at=now() WHERE id=$1`, request, coach)
	execSQLArgs(t, `UPDATE review_payments SET status='captured',captured_at=now() WHERE request_id=$1`, request)
	var payment string
	queryRowArgs(t, `SELECT id FROM review_payments WHERE request_id=$1`, []any{request}, &payment)
	execSQLArgs(t, `INSERT INTO coach_earnings(id,coach_id,source,review_request_id,gross_yen,share_percent,amount_yen) VALUES(gen_random_uuid(),$1,'review',$2,1500,70,1050)`, coach, request)
	return capturedFixture{h, f, admin, user, coach, payment, request, intent}
}
func (x capturedFixture) refund(t *testing.T) {
	t.Helper()
	rec := doJSON(t, x.h, http.MethodPost, "/v1/admin/coaching/payments/"+x.payment+"/refund", x.admin, map[string]any{"source": "review", "reason": "運営確認済み"})
	if rec.Code != 202 {
		t.Fatalf("refund %d %s", rec.Code, rec.Body.String())
	}
}
func (x capturedFixture) run(t *testing.T) {
	t.Helper()
	rec := doJSON(t, x.h, http.MethodPost, "/v1/admin/coaching/payouts/run", x.admin, nil)
	if rec.Code != 200 {
		t.Fatalf("run %d %s", rec.Code, rec.Body.String())
	}
}
func scalar(t *testing.T, q string, args ...any) int {
	t.Helper()
	var n int
	queryRowArgs(t, q, args, &n)
	return n
}
func (x capturedFixture) paid(t *testing.T) {
	t.Helper()
	execSQLArgs(t, `INSERT INTO coach_payouts(id,coach_id,month,amount_yen,status,stripe_transfer_id,paid_at) VALUES(gen_random_uuid(),$1,date '2020-01-01',1050,'paid','tr_historical',now())`, x.coach)
	execSQLArgs(t, `UPDATE coach_earnings SET payout_id=(SELECT id FROM coach_payouts WHERE coach_id=$1) WHERE coach_id=$1`, x.coach)
}
func (x capturedFixture) addEarning(t *testing.T, n int) {
	t.Helper()
	execSQLArgs(t, `INSERT INTO coach_earnings(id,coach_id,source,gross_yen,share_percent,amount_yen,created_at) VALUES(gen_random_uuid(),$1,'review',$2,70,$2,`+lastMonthJST+`)`, x.coach, n)
}

func TestCapturedRefundPendingSuccessAndAuthorization(t *testing.T) {
	x := newCapturedFixture(t)
	path := "/v1/admin/coaching/payments/" + x.payment + "/refund"
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		p := "/v1/admin/coaching/payments"
		if method == http.MethodPost {
			p = path
		}
		rec := doJSON(t, x.h, method, p, x.user, map[string]any{"source": "review", "reason": "test"})
		if rec.Code != 403 {
			t.Fatalf("authorization %d", rec.Code)
		}
	}
	x.f.mu.Lock()
	x.f.ticketRefundStatus = "pending"
	x.f.mu.Unlock()
	x.refund(t)
	x.refund(t)
	if reviewPaymentStatus(t, x.request) != "refund_pending" || len(x.f.refundList()) != 1 {
		t.Fatal("pending / duplicate refund")
	}
	// Pending refund must never be paid even before Stripe confirms success.
	execSQLArgs(t, `UPDATE coach_earnings SET created_at=now()-interval '3 months' WHERE coach_id=$1`, x.coach)
	x.run(t)
	if len(x.f.transferList()) != 0 {
		t.Fatal("pending refund paid")
	}
	rec := stripeWebhook(t, x.h, testWebhookSecret, "charge.refunded", map[string]any{"id": "ch_full", "currency": "jpy", "payment_intent": x.intent, "amount_refunded": 1500, "refunds": map[string]any{"data": []map[string]any{{"id": "re_1", "amount": 1500, "status": "succeeded"}}}})
	if rec.Code != 200 || reviewPaymentStatus(t, x.request) != "refunded" {
		t.Fatalf("sync %d %s", rec.Code, rec.Body.String())
	}
	x.refund(t)
	if len(x.f.refundList()) != 1 {
		t.Fatal("duplicate")
	}
	if scalar(t, `SELECT count(*) FROM captured_refund_earnings WHERE excluded`) != 1 {
		t.Fatal("unpaid exclusion")
	}
	x.run(t)
	if len(x.f.transferList()) != 0 {
		t.Fatal("refunded earning paid")
	}
	rec = doJSON(t, x.h, http.MethodGet, "/v1/admin/coaching/payments", x.admin, nil)
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var data struct {
		Refunds []struct {
			Reason  string `json:"reason"`
			AdminID string `json:"admin_id"`
		}
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil || len(data.Refunds) != 1 || data.Refunds[0].AdminID == "" || data.Refunds[0].Reason != "運営確認済み" {
		t.Fatalf("audit %s", rec.Body.String())
	}
}
func TestCapturedRefundDebtCarryAndNetThreshold(t *testing.T) {
	x := newCapturedFixture(t)
	x.paid(t)
	x.refund(t)
	if scalar(t, `SELECT sum(amount_yen-applied_yen) FROM coach_refund_adjustments`) != 1050 {
		t.Fatal("missing debt")
	}
	x.addEarning(t, 400)
	x.run(t)
	x.run(t)
	if len(x.f.transferList()) != 0 || scalar(t, `SELECT sum(amount_yen-applied_yen) FROM coach_refund_adjustments`) != 650 {
		t.Fatal("partial carry")
	}
	x.addEarning(t, 3650)
	x.run(t)
	// Net 3000 after clearing the remaining 650 debt, not gross 3650.
	transfers := x.f.transferList()
	if len(transfers) != 1 || transfers[0].Get("amount") != "3000" {
		t.Fatalf("net transfer %v", transfers)
	}
	if scalar(t, `SELECT sum(amount_yen-applied_yen) FROM coach_refund_adjustments`) != 0 {
		t.Fatal("unapplied debt")
	}
	if scalar(t, `SELECT count(*) FROM coach_refund_applications WHERE payout_id IS NULL`) != 0 {
		t.Fatal("missing payout lineage")
	}
	if scalar(t, `SELECT amount_yen FROM coach_payouts WHERE stripe_transfer_id='tr_historical'`) != 1050 {
		t.Fatal("history changed")
	}
	x.run(t)
	if len(x.f.transferList()) != 1 {
		t.Fatal("reapplied")
	}
}
func TestCapturedRefundNetBelowMinimumAndZero(t *testing.T) {
	for _, n := range []int{1050, 4000} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			x := newCapturedFixture(t)
			x.paid(t)
			x.refund(t)
			x.addEarning(t, n)
			x.run(t)
			if len(x.f.transferList()) != 0 {
				t.Fatal("zero or net below 3000 paid")
			}
			if scalar(t, `SELECT count(*) FROM coach_payouts WHERE stripe_transfer_id IS DISTINCT FROM 'tr_historical'`) != 0 {
				t.Fatal("zero payout created")
			}
			x.run(t)
			if scalar(t, `SELECT applied_yen FROM coach_refund_adjustments`) != 1050 {
				t.Fatal("double applied")
			}
		})
	}
}
func TestCapturedRefundDashboardIdempotentAndPartialAlert(t *testing.T) {
	x := newCapturedFixture(t)
	x.paid(t)
	notify := func(amount int, status string) {
		t.Helper()
		rec := stripeWebhook(t, x.h, testWebhookSecret, "refund.created", map[string]any{"id": "re_dashboard", "currency": "jpy", "payment_intent": x.intent, "amount": amount, "status": status})
		if rec.Code != 200 {
			t.Fatal(rec.Body.String())
		}
	}
	notify(500, "succeeded")
	if reviewPaymentStatus(t, x.request) != "captured" {
		t.Fatal("partial refund marked full")
	}
	notify(1500, "succeeded")
	notify(1500, "succeeded")
	notify(1500, "pending")
	if reviewPaymentStatus(t, x.request) != "refunded" || scalar(t, `SELECT count(*) FROM coach_refund_adjustments`) != 1 {
		t.Fatal("dashboard duplicate")
	}
}
func TestCapturedRefundFailureRestoresAndCanRetry(t *testing.T) {
	x := newCapturedFixture(t)
	x.f.mu.Lock()
	x.f.ticketRefundStatus = "failed"
	x.f.mu.Unlock()
	x.refund(t)
	if reviewPaymentStatus(t, x.request) != "captured" || scalar(t, `SELECT count(*) FROM captured_payment_refunds WHERE status='failed' AND failure IS NOT NULL`) != 1 {
		t.Fatal("failed refund not restored")
	}
	x.f.mu.Lock()
	x.f.ticketRefundStatus = "succeeded"
	x.f.mu.Unlock()
	x.refund(t)
	if reviewPaymentStatus(t, x.request) != "refunded" || len(x.f.refundList()) != 2 {
		t.Fatal("retry failed")
	}
}
func TestCapturedRefundConcurrentWithPayout(t *testing.T) {
	x := newCapturedFixture(t)
	execSQLArgs(t, `UPDATE coach_earnings SET created_at=now()-interval '3 months' WHERE coach_id=$1`, x.coach)
	x.addEarning(t, 4000)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				x.refund(t)
			} else {
				x.run(t)
			}
		}(i)
	}
	wg.Wait()
	if len(x.f.refundList()) != 1 {
		t.Fatal("concurrent duplicate refund")
	}
	total := 0
	for _, p := range x.f.transferList() {
		var n int
		_, _ = fmt.Sscan(p.Get("amount"), &n)
		total += n
	}
	debt := scalar(t, `SELECT COALESCE(sum(amount_yen-applied_yen),0) FROM coach_refund_adjustments`)
	if total-debt != 4000 {
		t.Fatalf("net inconsistency: paid %d outstanding %d", total, debt)
	}
	if scalar(t, `SELECT count(*) FROM captured_refund_earnings`) != 1 {
		t.Fatal("earning settled twice")
	}
}

func TestCapturedRefundConsultationSixHundred(t *testing.T) {
	x := newCapturedFixture(t)
	id := uuid.New()
	execSQLArgs(t, `INSERT INTO coaching_consultations(id,requester_id,coach_id,request_key,review_request_id,minor,funding,price_yen,share_percent,status,payment_state,payment_intent_id,paid_at,answered_at) SELECT $1,payer_id,$2,gen_random_uuid(),request_id,false,'cash',600,70,'answered','paid','pi_extra_600',now(),now() FROM review_payments WHERE id=$3`, id, x.coach, x.payment)
	execSQLArgs(t, `INSERT INTO coach_earnings(id,coach_id,source,consultation_id,gross_yen,share_percent,amount_yen) VALUES(gen_random_uuid(),$1,'consultation',$2,600,70,420)`, x.coach, id)
	rec := doJSON(t, x.h, "POST", "/v1/admin/coaching/payments/"+id.String()+"/refund", x.admin, map[string]any{"source": "consultation", "reason": "追加相談の返金"})
	if rec.Code != 202 {
		t.Fatal(rec.Body.String())
	}
	if len(x.f.refundList()) != 1 || x.f.refundList()[0].Get("amount") != "600" {
		t.Fatal("consultation amount")
	}
	if scalar(t, `SELECT count(*) FROM captured_refund_earnings f JOIN coach_earnings e ON e.id=f.earning_id WHERE e.consultation_id=$1 AND f.excluded`, id) != 1 {
		t.Fatal("consultation earning not excluded")
	}
	if reviewPaymentStatus(t, x.request) != "captured" {
		t.Fatal("unrelated payment changed")
	}
}
func TestCapturedRefundDashboardPendingNeverCreatesNewRefund(t *testing.T) {
	x := newCapturedFixture(t)
	x.f.mu.Lock()
	x.f.refunds = append(x.f.refunds, url.Values{"payment_intent": {x.intent}, "amount": {"1500"}})
	x.f.ticketRefundStatus = "pending"
	x.f.mu.Unlock()
	rec := stripeWebhook(t, x.h, testWebhookSecret, "refund.created", map[string]any{"id": "re_1", "currency": "jpy", "payment_intent": x.intent, "amount": 1500, "status": "pending"})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	x.refund(t)
	if len(x.f.refundList()) != 1 || reviewPaymentStatus(t, x.request) != "refund_pending" {
		t.Fatal("dashboard refund duplicated")
	}
	x.f.mu.Lock()
	x.f.ticketRefundStatus = "succeeded"
	x.f.mu.Unlock()
	x.refund(t)
	if len(x.f.refundList()) != 1 || reviewPaymentStatus(t, x.request) != "refunded" {
		t.Fatal("dashboard pending not reconciled")
	}
}
func TestCapturedRefundAfterRequestDeletion(t *testing.T) {
	x := newCapturedFixture(t)
	x.paid(t)
	execSQLArgs(t, `DELETE FROM review_requests WHERE id=$1`, x.request)
	x.refund(t)
	if scalar(t, `SELECT sum(amount_yen) FROM coach_refund_adjustments`) != 1050 {
		t.Fatal("lost earning link after deletion")
	}
}
func TestCapturedRefundRecoversAfterDatabaseFailure(t *testing.T) {
	x := newCapturedFixture(t)
	execSQLArgs(t, `CREATE OR REPLACE FUNCTION test_captured_refund_failure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test database failure'; END $$`)
	execSQLArgs(t, `CREATE TRIGGER test_captured_refund_failure BEFORE UPDATE ON captured_payment_refunds FOR EACH ROW WHEN (NEW.status='succeeded') EXECUTE FUNCTION test_captured_refund_failure()`)
	t.Cleanup(func() {
		execSQLArgs(t, `DROP TRIGGER IF EXISTS test_captured_refund_failure ON captured_payment_refunds`)
		execSQLArgs(t, `DROP FUNCTION IF EXISTS test_captured_refund_failure()`)
	})
	x.refund(t)
	if reviewPaymentStatus(t, x.request) != "refund_pending" {
		t.Fatal("ambiguous success lost")
	}
	execSQLArgs(t, `DROP TRIGGER test_captured_refund_failure ON captured_payment_refunds`)
	x.refund(t)
	if len(x.f.refundList()) != 1 || reviewPaymentStatus(t, x.request) != "refunded" {
		t.Fatal("database retry created duplicate")
	}
}

func TestCapturedRefundRestoresDebtWhenPersonalEarningIsRefunded(t *testing.T) {
	x := newCapturedFixture(t)
	x.paid(t)
	x.refund(t)
	contract := uuid.New()
	execSQLArgs(t, `INSERT INTO coach_earnings(id,coach_id,source,contract_id,gross_yen,share_percent,amount_yen,created_at) VALUES(gen_random_uuid(),$1,'personal',$2,500,80,400,`+lastMonthJST+`)`, x.coach, contract)
	x.run(t)
	if scalar(t, `SELECT applied_yen FROM coach_refund_adjustments`) != 400 {
		t.Fatal("personal offset missing")
	}
	execSQLArgs(t, `DELETE FROM coach_earnings WHERE contract_id=$1 AND payout_id IS NULL`, contract)
	if scalar(t, `SELECT applied_yen FROM coach_refund_adjustments`) != 0 {
		t.Fatal("refunded consideration did not restore debt")
	}
	if scalar(t, `SELECT count(*) FROM coach_refund_applications`) != 0 {
		t.Fatal("deleted earning application retained")
	}
}

func TestCapturedRefundOrphanRequiresReconciliation(t *testing.T) {
	x := newCapturedFixture(t)
	execSQLArgs(t, `DELETE FROM review_payment_earning_links WHERE payment_id=$1`, x.payment)
	execSQLArgs(t, `DELETE FROM review_requests WHERE id=$1`, x.request)
	rec := doJSON(t, x.h, "POST", "/v1/admin/coaching/payments/"+x.payment+"/refund", x.admin, map[string]any{"source": "review", "reason": "test"})
	if rec.Code == 202 || len(x.f.refundList()) != 0 {
		t.Fatal("orphaned payment refunded without reconciliation")
	}
	rec = stripeWebhook(t, x.h, testWebhookSecret, "refund.created", map[string]any{"id": "re_orphan", "currency": "jpy", "payment_intent": x.intent, "amount": 1500, "status": "succeeded"})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var status string
	queryRowArgs(t, `SELECT status FROM review_payments WHERE id=$1`, []any{x.payment}, &status)
	if status != "captured" {
		t.Fatal("orphan silently synchronized")
	}
}
