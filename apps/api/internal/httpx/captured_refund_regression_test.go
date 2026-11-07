package httpx_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/auth"
	"github.com/hinata/track-talk/apps/api/internal/httpx"
	"github.com/hinata/track-talk/apps/api/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func lateRefundEvent(t *testing.T, x capturedFixture, typ, status string) {
	t.Helper()
	rec := stripeWebhook(t, x.h, testWebhookSecret, typ, map[string]any{"id": "re_1", "currency": "jpy", "payment_intent": x.intent, "amount": 1500, "status": status, "failure_reason": "declined"})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
}
func TestCapturedLateFailureCompensation(t *testing.T) {
	for _, scenario := range []string{"unpaid", "unapplied", "partial", "paid"} {
		t.Run(scenario, func(t *testing.T) {
			x := newCapturedFixture(t)
			if scenario != "unpaid" {
				x.paid(t)
			}
			x.refund(t)
			if scenario == "partial" {
				x.addEarning(t, 400)
				x.run(t)
			}
			if scenario == "paid" {
				id := uuid.New()
				var adjustment uuid.UUID
				queryRowArgs(t, `SELECT id FROM coach_refund_adjustments`, nil, &adjustment)
				payout := uuid.New()
				execSQLArgs(t, `INSERT INTO coach_payouts(id,coach_id,month,amount_yen,status,stripe_transfer_id,paid_at) VALUES($1,$2,date '2020-02-01',3000,'paid','tr_offset_historical',now())`, payout, x.coach)
				execSQLArgs(t, `INSERT INTO coach_earnings(id,coach_id,source,gross_yen,share_percent,amount_yen,payout_id, confirmed_at) VALUES($1,$2,'review',4050,70,4050,$3, now())`, id, x.coach, payout)
				execSQLArgs(t, `INSERT INTO coach_refund_applications(id,adjustment_id,earning_id,amount_yen,payout_id) VALUES(gen_random_uuid(),$1,$2,1050,$3)`, adjustment, id, payout)
				execSQLArgs(t, `UPDATE coach_refund_adjustments SET applied_yen=1050 WHERE id=$1`, adjustment)
			}
			lateRefundEvent(t, x, "refund.failed", "failed")
			lateRefundEvent(t, x, "refund.failed", "failed")
			lateRefundEvent(t, x, "refund.updated", "succeeded")
			if reviewPaymentStatus(t, x.request) != "captured" {
				t.Fatal("late failure lost or old success reapplied")
			}
			if scalar(t, `SELECT count(*) FROM captured_payment_refunds WHERE status='failed' AND failure LIKE '%declined%'`) != 1 {
				t.Fatal("failure reason missing")
			}
			wantCredit, wantCancel := 0, 0
			switch scenario {
			case "unapplied":
				wantCancel = 1050
			case "partial":
				wantCancel = 650
				wantCredit = 400
			case "paid":
				wantCredit = 1050
			}
			if scalar(t, `SELECT COALESCE(sum(amount_yen),0) FROM coach_refund_compensations`) != wantCredit || scalar(t, `SELECT COALESCE(sum(amount_yen),0) FROM coach_refund_cancellations`) != wantCancel {
				t.Fatal("incorrect compensation")
			}
			if scalar(t, `SELECT count(*) FROM captured_refund_earnings WHERE excluded`) != 0 {
				t.Fatal("exclusion not restored")
			}
			if scenario == "paid" {
				if scalar(t, `SELECT amount_yen FROM coach_payouts WHERE stripe_transfer_id='tr_offset_historical'`) != 3000 {
					t.Fatal("historical payout changed")
				}
				execSQLArgs(t, `UPDATE coach_earnings SET confirmed_at=now()-interval '3 months' WHERE source='refund_compensation'`)
				x.run(t)
				if len(x.f.transferList()) != 1 || x.f.transferList()[0].Get("amount") != "1050" {
					t.Fatal("compensation not paid monthly")
				}
			}
			if scenario == "partial" {
				execSQLArgs(t, `DELETE FROM coach_earnings WHERE payout_id IS NULL AND source='review' AND review_request_id IS NULL`)
				if scalar(t, `SELECT COALESCE(sum(amount_yen),0) FROM coach_refund_adjustments WHERE kind='compensation_recovery'`) != 400 {
					t.Fatal("canceled debt resurrected instead of recovering credit")
				}
			}
			x.refund(t)
			if reviewPaymentStatus(t, x.request) != "refunded" || len(x.f.refundList()) != 2 {
				t.Fatal("new operation retry failed")
			}
		})
	}
}
func TestCapturedSuccessfulRefundIsReconciled(t *testing.T) {
	x := newCapturedFixture(t)
	x.paid(t)
	x.refund(t)
	x.f.mu.Lock()
	x.f.ticketRefundStatus = "failed"
	x.f.mu.Unlock()
	if rec := tick(t, x.h, "tick-secret"); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if reviewPaymentStatus(t, x.request) != "captured" || scalar(t, `SELECT sum(amount_yen) FROM coach_refund_cancellations`) != 1050 {
		t.Fatal("successful refund not polled")
	}
}
func migrationSQL(t *testing.T, name string) string {
	t.Helper()
	b, err := migrations.FS.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func migrationConn(t *testing.T) *pgx.Conn {
	t.Helper()
	c, err := pgx.Connect(context.Background(), testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}
func TestCapturedMigrationDownGuard(t *testing.T) {
	x := newCapturedFixture(t)
	x.refund(t)
	c := migrationConn(t)
	for _, n := range []string{"000128_refund_failure_compensation.down.sql", "000127_captured_payment_refunds.down.sql"} {
		if _, err := c.Exec(context.Background(), migrationSQL(t, n)); err == nil || !strings.Contains(err.Error(), "rollback refused") {
			t.Fatalf("unguarded down: %v", err)
		}
	}
	if scalar(t, `SELECT count(*) FROM captured_payment_refunds`) != 1 {
		t.Fatal("ledger destroyed")
	}
}
func TestCapturedMigrationEmptyRoundtripAndBackfill(t *testing.T) {
	x := newCapturedFixture(t)
	c := migrationConn(t)
	ctx := context.Background()
	tx, err := c.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	// No refund accounting exists; payment/earning association alone permits rollback.
	for _, n := range []string{"000128_refund_failure_compensation.down.sql", "000127_captured_payment_refunds.down.sql"} {
		if _, err := tx.Exec(ctx, migrationSQL(t, n)); err != nil {
			t.Fatal(err)
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO review_payments(id,request_id,payer_id,method,amount_yen,checkout_session_id,checkout_url,session_expires_at,payment_intent_id,status,authorized_at,captured_at,refunded_at) SELECT gen_random_uuid(),request_id,payer_id,'card',1500,'cs_old_'||gen_random_uuid(),'https://test.invalid',now(),'pi_old_'||gen_random_uuid(),'refunded',now()-interval '2 days',now()-interval '2 days',now()-interval '1 day' FROM review_payments WHERE id=$1`, x.payment)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"000127_captured_payment_refunds.up.sql", "000128_refund_failure_compensation.up.sql"} {
		if _, err := tx.Exec(ctx, migrationSQL(t, n)); err != nil {
			t.Fatal(err)
		}
	}
	var picked string
	if err := tx.QueryRow(ctx, `SELECT payment_id FROM review_payment_earning_links`).Scan(&picked); err != nil || picked != x.payment {
		t.Fatalf("wrong earning attribution: %v", err)
	}
}
func TestCapturedAdminPaginationBeyondTwoHundred(t *testing.T) {
	x := newCapturedFixture(t)
	x.paid(t)
	x.refund(t)
	var original string
	queryRowArgs(t, `SELECT id FROM captured_payment_refunds`, nil, &original)
	execSQLArgs(t, `UPDATE captured_payment_refunds SET created_at=now()-interval '2 years'; UPDATE coach_refund_adjustments SET created_at=now()-interval '2 years'`)
	execSQLArgs(t, `WITH f AS (INSERT INTO captured_payment_refunds(id,source,payment_id,payment_intent_id,amount_yen,reason,origin,status) SELECT gen_random_uuid(),'review',gen_random_uuid(),'pi_synthetic_'||n,1500,'pagination fixture','stripe','failed' FROM generate_series(1,205) n RETURNING id),a AS (INSERT INTO coach_refund_adjustments(id,refund_id,coach_id,kind,amount_yen) SELECT gen_random_uuid(),id,$1,'compensation_recovery',1050 FROM f RETURNING id,refund_id) INSERT INTO coach_refund_cancellations(id,adjustment_id,refund_id,amount_yen) SELECT gen_random_uuid(),id,refund_id,1050 FROM a`, x.coach)
	found := false
	for page := 1; page < 15; page++ {
		rec := doJSON(t, x.h, "GET", fmtPagination(page), x.admin, nil)
		if rec.Code != 200 {
			t.Fatal(rec.Body.String())
		}
		var d struct {
			Adjustments []struct {
				RefundID string `json:"refund_id"`
			}
			More bool `json:"has_more"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
			t.Fatal(err)
		}
		for _, a := range d.Adjustments {
			if a.RefundID == original {
				found = true
			}
		}
		if !d.More {
			break
		}
	}
	if !found {
		t.Fatal("old adjustment unreachable")
	}
	rec := doJSON(t, x.h, "GET", "/v1/admin/coaching/payments?coach_id="+x.coach.String()+"&unapplied=true", x.admin, nil)
	var d struct {
		Adjustments []struct {
			Remaining int    `json:"remaining_yen"`
			RefundID  string `json:"refund_id"`
		}
		Refunds []struct {
			ID string `json:"id"`
		}
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil || len(d.Adjustments) != 1 || d.Adjustments[0].Remaining != 1050 {
		t.Fatalf("unapplied filter %s", rec.Body.String())
	}
	rec = doJSON(t, x.h, "GET", "/v1/admin/coaching/payments?refund_id="+original, x.admin, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &d)
	if len(d.Refunds) != 1 || d.Refunds[0].ID != original {
		t.Fatal("origin audit unreachable")
	}
}
func fmtPagination(page int) string { return "/v1/admin/coaching/payments?page=" + fmt.Sprint(page) }
func TestCapturedMultipleCoachesDatabaseFailureAndRestart(t *testing.T) {
	x := newCapturedFixture(t)
	second, _ := onDutyCoach(t, x.h, x.admin)
	execSQLArgs(t, `UPDATE coach_earnings SET confirmed_at=now()-interval '3 months'`)
	x.addEarning(t, 4000)
	execSQLArgs(t, `INSERT INTO coach_earnings(id,coach_id,source,gross_yen,share_percent,amount_yen,confirmed_at) VALUES(gen_random_uuid(),$1,'review',4000,70,4000,now()-interval '3 months')`, second)
	execSQLArgs(t, `CREATE OR REPLACE FUNCTION test_second_payout_failure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF (SELECT count(*) FROM coach_payouts WHERE status='paid')>=1 THEN RAISE EXCEPTION 'second payout update failed'; END IF; RETURN NEW; END $$`)
	execSQLArgs(t, `CREATE TRIGGER test_second_payout_failure BEFORE UPDATE ON coach_payouts FOR EACH ROW WHEN (NEW.status='paid') EXECUTE FUNCTION test_second_payout_failure()`)
	t.Cleanup(func() {
		execSQLArgs(t, `DROP TRIGGER IF EXISTS test_second_payout_failure ON coach_payouts`)
		execSQLArgs(t, `DROP FUNCTION IF EXISTS test_second_payout_failure()`)
	})
	x.f.mu.Lock()
	x.f.transferDelay = 20 * time.Millisecond
	x.f.mu.Unlock()
	rec := doJSON(t, x.h, "POST", "/v1/admin/coaching/payouts/run", x.admin, nil)
	if rec.Code == 200 || len(x.f.transferList()) != 2 || scalar(t, `SELECT count(*) FROM coach_payouts WHERE status='paid'`) != 0 {
		t.Fatal("did not exercise partial external success with rolled back database")
	}
	execSQLArgs(t, `DROP TRIGGER test_second_payout_failure ON coach_payouts`)
	pool, err := pgxpool.New(context.Background(), testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	v, err := auth.NewVerifier("test-jwt-secret-for-ci-only", "", "authenticated")
	if err != nil {
		t.Fatal(err)
	}
	x.h = httpx.New(x.f.serverConfig, pool, v) // new API instance, same Stripe and durable DB
	x.run(t)
	if len(x.f.transferList()) != 2 || scalar(t, `SELECT count(*) FROM coach_payouts WHERE status='paid'`) != 2 {
		t.Fatal("restart double-paid or failed to reconcile transfer_group")
	}
}

func TestCapturedCreatedFailureReasonAndFailedTerminal(t *testing.T) {
	x := newCapturedFixture(t)
	lateRefundEvent(t, x, "refund.created", "failed")
	lateRefundEvent(t, x, "refund.updated", "succeeded")
	if reviewPaymentStatus(t, x.request) != "captured" || scalar(t, `SELECT count(*) FROM captured_payment_refunds WHERE status='failed' AND failure LIKE '%declined%'`) != 1 {
		t.Fatal("failed created event or terminal ordering lost")
	}
}

func refundFailureNull(t *testing.T, id uuid.UUID) bool {
	t.Helper()
	var isNull bool
	queryRowArgs(t, `SELECT failure IS NULL FROM captured_payment_refunds WHERE id=$1`, []any{id}, &isNull)
	return isNull
}

func refundFailureText(t *testing.T, id uuid.UUID) string {
	t.Helper()
	var s string
	queryRowArgs(t, `SELECT COALESCE(failure,'') FROM captured_payment_refunds WHERE id=$1`, []any{id}, &s)
	return s
}

func refundStatus(t *testing.T, id uuid.UUID) string {
	t.Helper()
	var s string
	queryRowArgs(t, `SELECT status FROM captured_payment_refunds WHERE id=$1`, []any{id}, &s)
	return s
}

func adminRefundFailure(t *testing.T, x capturedFixture, id uuid.UUID) *string {
	t.Helper()
	rec := doJSON(t, x.h, http.MethodGet, "/v1/admin/coaching/payments?refund_id="+id.String(), x.admin, nil)
	if rec.Code != 200 {
		t.Fatalf("admin payments %d %s", rec.Code, rec.Body.String())
	}
	var d struct {
		Refunds []struct {
			ID      string  `json:"id"`
			Failure *string `json:"failure"`
		} `json:"refunds"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil || len(d.Refunds) != 1 || d.Refunds[0].ID != id.String() {
		t.Fatalf("admin audit %s", rec.Body.String())
	}
	return d.Refunds[0].Failure
}

// TestCapturedTransientWarningClearsAfterRecovery は N1 の回帰試験である。
// 成功済み返金の再照合が一時的に失敗すると tick が一時警告を載せ、復旧後の
// 同一 ID・金額の succeeded 取得で警告が消え、未復旧のままでは残る。
func TestCapturedTransientWarningClearsAfterRecovery(t *testing.T) {
	x := newCapturedFixture(t)
	x.paid(t)
	x.refund(t)
	var id uuid.UUID
	queryRowArgs(t, `SELECT id FROM captured_payment_refunds`, nil, &id)
	var remote string
	queryRowArgs(t, `SELECT COALESCE(stripe_refund_id,'') FROM captured_payment_refunds WHERE id=$1`, []any{id}, &remote)
	if remote == "" || !refundFailureNull(t, id) {
		t.Fatal("succeeded refund missing remote identity or unexpectedly warned")
	}
	var succeededAt time.Time
	queryRowArgs(t, `SELECT succeeded_at FROM captured_payment_refunds WHERE id=$1`, []any{id}, &succeededAt)

	// 一時的な照合失敗に見せかける。成功済みの既知返金が Stripe 側に見えないと
	// process は照合要求エラーを返し、tick が一時警告を載せる。
	x.f.mu.Lock()
	saved := append([]url.Values(nil), x.f.refunds...)
	x.f.refunds = nil
	x.f.mu.Unlock()
	if rec := tick(t, x.h, "tick-secret"); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if refundStatus(t, id) != "succeeded" || refundFailureText(t, id) != "Stripe reconciliation required" || reviewPaymentStatus(t, x.request) != "refunded" {
		t.Fatal("transient failure did not warn without changing ledger state")
	}
	if got := adminRefundFailure(t, x, id); got == nil || *got != "Stripe reconciliation required" {
		t.Fatal("admin display missing transient warning")
	}

	// (c) 復旧していないまま tick しても警告は残る。
	if rec := tick(t, x.h, "tick-secret"); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if refundFailureNull(t, id) {
		t.Fatal("unrecovered warning disappeared")
	}

	// (b) Stripe 側が復旧し、同一 ID・金額の succeeded 取得できたら警告が消える。
	x.f.mu.Lock()
	x.f.refunds = saved
	x.f.mu.Unlock()
	if rec := tick(t, x.h, "tick-secret"); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if !refundFailureNull(t, id) || refundStatus(t, id) != "succeeded" || reviewPaymentStatus(t, x.request) != "refunded" {
		t.Fatal("recovered warning remained or ledger state changed")
	}
	var after string
	queryRowArgs(t, `SELECT COALESCE(stripe_refund_id,'') FROM captured_payment_refunds WHERE id=$1`, []any{id}, &after)
	if after != remote {
		t.Fatal("remote identity changed on recovery")
	}
	var recoveredAt time.Time
	queryRowArgs(t, `SELECT succeeded_at FROM captured_payment_refunds WHERE id=$1`, []any{id}, &recoveredAt)
	if !recoveredAt.Equal(succeededAt) {
		t.Fatal("success timestamp moved on recovery")
	}
	if got := adminRefundFailure(t, x, id); got != nil {
		t.Fatal("admin display kept recovered warning")
	}

	// 銀行由来の失敗理由は一時警告と区別し、復旧成功でも消さない。
	execSQLArgs(t, `UPDATE captured_payment_refunds SET failure='Stripe refund failed: card_declined' WHERE id=$1`, id)
	if rec := tick(t, x.h, "tick-secret"); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if refundFailureText(t, id) != "Stripe refund failed: card_declined" {
		t.Fatal("permanent failure reason cleared as transient")
	}
}
