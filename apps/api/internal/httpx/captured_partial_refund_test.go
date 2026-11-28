package httpx_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
)

// 本体 #1097：Stripe ダッシュボードの部分返金の同期（G1）と、日次照合の自己修復（G2）。
// 固定の支払いは 1,500円・報酬 1,050円（legacyReviewAmount）。報酬の減額の目標額は floor(1050 × 返金累計 ÷ 1500)。

const partialRefundTitle = "スタンダードの代金の一部を返金しました"

func (x capturedFixture) dashboard(t *testing.T, typ, id string, amount int, status string) {
	t.Helper()
	rec := stripeWebhook(t, x.h, testWebhookSecret, typ, map[string]any{"id": id, "currency": "jpy", "payment_intent": x.intent, "amount": amount, "status": status})
	if rec.Code != http.StatusOK {
		t.Fatalf("webhook %d %s", rec.Code, rec.Body.String())
	}
}

// newCapturedFixtureInto は、同じサーバー・同じコーチで、別の選手の確定済みカード支払い（1,500円・報酬1,050円）を作る。
func newCapturedFixtureInto(t *testing.T, x capturedFixture) capturedFixture {
	t.Helper()
	_, user := demoRequester(t, x.h, x.admin)
	intent := "pi_captured_" + uuid.NewString()
	request := paidReview(t, x.h, user, "card", intent)
	legacyReviewAmount(t, request)
	execSQLArgs(t, `UPDATE review_requests SET coach_id=$2,status='returned',returned_at=now() WHERE id=$1`, request, x.coach)
	execSQLArgs(t, `UPDATE review_payments SET status='captured',captured_at=now() WHERE request_id=$1`, request)
	var payment string
	queryRowArgs(t, `SELECT id FROM review_payments WHERE request_id=$1`, []any{request}, &payment)
	execSQLArgs(t, `INSERT INTO coach_earnings(id,coach_id,source,review_request_id,gross_yen,share_percent,amount_yen, confirmed_at) VALUES(gen_random_uuid(),$1,'review',$2,1500,70,1050, now())`, x.coach, request)
	return capturedFixture{x.h, x.f, x.admin, user, x.coach, payment, request, intent}
}

// doRawSecurityCheck は、定期の見回り（日次の Stripe 照合を含む）を呼ぶ。
func doRawSecurityCheck(t *testing.T, h http.Handler, secret string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/internal/security/check", nil)
	req.Header.Set("X-Security-Check-Secret", secret)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func (x capturedFixture) refundedYen(t *testing.T) int {
	t.Helper()
	return scalar(t, `SELECT refunded_yen FROM review_payments WHERE id=$1`, x.payment)
}

// netOffset は、その支払いの返金で生きている相殺の合計（失敗した返金の相殺は取消・補償で打ち消される）。
func netOffset(t *testing.T) int {
	t.Helper()
	return scalar(t, `SELECT COALESCE(sum(a.amount_yen),0) FROM coach_refund_adjustments a JOIN captured_payment_refunds f ON f.id=a.refund_id WHERE f.status='succeeded' AND a.kind='refund_offset'`)
}

func (x capturedFixture) eligible(t *testing.T) {
	t.Helper()
	execSQLArgs(t, `UPDATE coach_earnings SET confirmed_at=`+lastMonthJST+` WHERE coach_id=$1`, x.coach)
}

func transferTotal(t *testing.T, f *fakeStripe) int {
	t.Helper()
	total := 0
	for _, p := range f.transferList() {
		n := 0
		for _, c := range p.Get("amount") {
			n = n*10 + int(c-'0')
		}
		total += n
	}
	return total
}

func titledNotices(t *testing.T, h http.Handler, tok, title string) int {
	t.Helper()
	n := 0
	for _, note := range coachingNotifications(t, h, tok) {
		if note.Title == title {
			n++
		}
	}
	return n
}

// T1（未送金）：部分返金で partially_refunded・累計がその額になり、未送金の報酬は按分額だけ減って送金される。
func TestCapturedPartialRefundUnpaidEarning(t *testing.T) {
	x := newCapturedFixture(t)
	x.eligible(t)
	x.dashboard(t, "refund.created", "re_part", 500, "succeeded")
	if reviewPaymentStatus(t, x.request) != "partially_refunded" || x.refundedYen(t) != 500 {
		t.Fatalf("partial state %s %d", reviewPaymentStatus(t, x.request), x.refundedYen(t))
	}
	if netOffset(t) != 350 {
		t.Fatalf("offset %d, want floor(1050*500/1500)=350", netOffset(t))
	}
	if titledNotices(t, x.h, x.user, partialRefundTitle) != 1 || titledNotices(t, x.h, x.user, "スタンダードの返金が完了しました") != 0 {
		t.Fatal("partial refund notice")
	}
	var body string
	queryRowArgs(t, `SELECT body FROM notifications WHERE title=$1`, []any{partialRefundTitle}, &body)
	if !strings.Contains(body, "500") {
		t.Fatalf("partial notice must show the amount: %q", body)
	}
	x.addEarning(t, 4000)
	x.run(t)
	x.run(t)
	// 1050 + 4000 − 350。減額は一度だけ。
	if got := transferTotal(t, x.f); got != 4700 {
		t.Fatalf("transfer %d, want 4700", got)
	}
	if scalar(t, `SELECT COALESCE(sum(amount_yen-applied_yen),0) FROM coach_refund_adjustments`) != 0 {
		t.Fatal("unapplied debt")
	}
	if scalar(t, `SELECT count(*) FROM captured_refund_earnings`) != 0 {
		t.Fatal("partial refund excluded the whole earning")
	}
}

// T1（送金済み）：送金済みの報酬は、按分額を相殺台帳で次の送金から差し引く。過去の送金は書き換えない。
func TestCapturedPartialRefundPaidEarning(t *testing.T) {
	x := newCapturedFixture(t)
	x.paid(t)
	x.dashboard(t, "refund.created", "re_part", 500, "succeeded")
	if reviewPaymentStatus(t, x.request) != "partially_refunded" || netOffset(t) != 350 {
		t.Fatal("paid partial offset")
	}
	x.addEarning(t, 4000)
	x.run(t)
	if got := transferTotal(t, x.f); got != 3650 {
		t.Fatalf("transfer %d, want 3650", got)
	}
	if scalar(t, `SELECT amount_yen FROM coach_payouts WHERE stripe_transfer_id='tr_historical'`) != 1050 {
		t.Fatal("history changed")
	}
}

// T2：部分返金を2回に分けて累計が支払額に達すると refunded。端数を含めて減額の総額は報酬額ちょうど。
func TestCapturedPartialRefundSplitReachesFull(t *testing.T) {
	for _, paid := range []bool{false, true} {
		name := "unpaid"
		if paid {
			name = "paid"
		}
		t.Run(name, func(t *testing.T) {
			x := newCapturedFixture(t)
			if paid {
				x.paid(t)
			} else {
				x.eligible(t)
			}
			x.dashboard(t, "refund.created", "re_a", 333, "succeeded")
			// floor(1050 × 333 ÷ 1500) = 233
			if netOffset(t) != 233 || reviewPaymentStatus(t, x.request) != "partially_refunded" {
				t.Fatalf("first offset %d", netOffset(t))
			}
			x.dashboard(t, "refund.created", "re_b", 1167, "succeeded")
			if reviewPaymentStatus(t, x.request) != "refunded" || x.refundedYen(t) != 1500 {
				t.Fatal("split refund not full")
			}
			if netOffset(t) != 1050 {
				t.Fatalf("offset total %d, want exactly 1050", netOffset(t))
			}
			if titledNotices(t, x.h, x.user, partialRefundTitle) != 1 || titledNotices(t, x.h, x.user, "スタンダードの返金が完了しました") != 1 {
				t.Fatal("notices for split refund")
			}
			x.addEarning(t, 5000)
			x.run(t)
			// 未送金：1050 + 5000 − 1050。送金済み：5000 − 1050。
			want := 5000
			if paid {
				want = 3950
			}
			if got := transferTotal(t, x.f); got != want {
				t.Fatalf("transfer %d, want %d", got, want)
			}
		})
	}
}

// T3：同じ refund.created／refund.updated の重複・順不同（charge.refunded を含む）でも、累計・減額・通知は二重にならない。
func TestCapturedPartialRefundIdempotentOutOfOrder(t *testing.T) {
	x := newCapturedFixture(t)
	x.paid(t)
	x.dashboard(t, "refund.updated", "re_p", 300, "pending")
	if reviewPaymentStatus(t, x.request) != "captured" || x.refundedYen(t) != 0 || netOffset(t) != 0 {
		t.Fatal("pending partial counted")
	}
	x.dashboard(t, "refund.updated", "re_p", 300, "succeeded")
	x.dashboard(t, "refund.created", "re_p", 300, "pending")
	x.dashboard(t, "refund.created", "re_p", 300, "succeeded")
	rec := stripeWebhook(t, x.h, testWebhookSecret, "charge.refunded", map[string]any{"id": "ch_p", "currency": "jpy", "payment_intent": x.intent, "amount_refunded": 300,
		"refunds": map[string]any{"data": []map[string]any{{"id": "re_p", "amount": 300, "status": "succeeded"}}}})
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	if x.refundedYen(t) != 300 || reviewPaymentStatus(t, x.request) != "partially_refunded" {
		t.Fatal("duplicate partial counted twice")
	}
	if scalar(t, `SELECT count(*) FROM captured_payment_refunds`) != 1 || netOffset(t) != 210 {
		t.Fatalf("offset %d, want 210", netOffset(t))
	}
	if titledNotices(t, x.h, x.user, partialRefundTitle) != 1 {
		t.Fatal("partial notice duplicated")
	}
}

// T4：成功した部分返金が後から失敗になると累計から外れ、按分の相殺が取り消される。
func TestCapturedPartialRefundLateFailure(t *testing.T) {
	t.Run("only_partial_unapplied", func(t *testing.T) {
		x := newCapturedFixture(t)
		x.paid(t)
		x.dashboard(t, "refund.created", "re_f", 500, "succeeded")
		x.dashboard(t, "refund.updated", "re_f", 500, "failed")
		if reviewPaymentStatus(t, x.request) != "captured" || x.refundedYen(t) != 0 || netOffset(t) != 0 {
			t.Fatal("failed partial not restored")
		}
		if scalar(t, `SELECT sum(amount_yen) FROM coach_refund_cancellations`) != 350 {
			t.Fatal("offset not canceled")
		}
		if titledNotices(t, x.h, x.user, partialRefundTitle) != 1 {
			t.Fatal("delivered notice must stay as history")
		}
	})
	t.Run("other_partial_remains", func(t *testing.T) {
		x := newCapturedFixture(t)
		x.paid(t)
		x.dashboard(t, "refund.created", "re_f1", 500, "succeeded")
		x.dashboard(t, "refund.created", "re_f2", 300, "succeeded")
		// floor(1050 × 800 ÷ 1500) = 560
		if netOffset(t) != 560 {
			t.Fatalf("offset %d", netOffset(t))
		}
		x.dashboard(t, "refund.failed", "re_f1", 500, "failed")
		if reviewPaymentStatus(t, x.request) != "partially_refunded" || x.refundedYen(t) != 300 {
			t.Fatal("remaining partial lost")
		}
		// floor(1050 × 300 ÷ 1500) = 210
		if netOffset(t) != 210 {
			t.Fatalf("offset after failure %d, want 210", netOffset(t))
		}
	})
	t.Run("applied_then_failed_compensates", func(t *testing.T) {
		x := newCapturedFixture(t)
		x.paid(t)
		x.dashboard(t, "refund.created", "re_f", 500, "succeeded")
		x.addEarning(t, 4000)
		x.run(t)
		if transferTotal(t, x.f) != 3650 {
			t.Fatal("offset not applied")
		}
		x.dashboard(t, "refund.updated", "re_f", 500, "failed")
		if scalar(t, `SELECT sum(amount_yen) FROM coach_refund_compensations`) != 350 || reviewPaymentStatus(t, x.request) != "captured" {
			t.Fatal("applied offset not compensated")
		}
	})
}

// T5：追加質問（cash・paid）でも部分返金を同期する。回答前に一部返金した追加質問は、回答で記録する報酬もその時点で減らす。
func TestCapturedPartialRefundConsultation(t *testing.T) {
	x := newCapturedFixture(t)
	answered, unanswered := uuid.New(), uuid.New()
	for _, c := range []struct {
		id     uuid.UUID
		status string
	}{{answered, "answered"}, {unanswered, "in_review"}} {
		execSQLArgs(t, `INSERT INTO coaching_consultations(id,requester_id,coach_id,request_key,review_request_id,minor,funding,price_yen,share_percent,status,payment_state,payment_intent_id,paid_at,answered_at) SELECT $1,payer_id,$2,gen_random_uuid(),request_id,false,'cash',500,70,$4,'paid',$3,now(),CASE WHEN $4='answered' THEN now() END FROM review_payments WHERE id=$5`, c.id, x.coach, "pi_extra_"+c.id.String(), c.status, x.payment)
	}
	execSQLArgs(t, `INSERT INTO coach_earnings(id,coach_id,source,consultation_id,gross_yen,share_percent,amount_yen, confirmed_at) VALUES(gen_random_uuid(),$1,'consultation',$2,500,70,350, now())`, x.coach, answered)
	for _, id := range []uuid.UUID{answered, unanswered} {
		rec := stripeWebhook(t, x.h, testWebhookSecret, "refund.created", map[string]any{"id": "re_c_" + id.String(), "currency": "jpy", "payment_intent": "pi_extra_" + id.String(), "amount": 200, "status": "succeeded"})
		if rec.Code != http.StatusOK {
			t.Fatal(rec.Body.String())
		}
		var state string
		var refunded int
		queryRowArgs(t, `SELECT payment_state,refunded_yen FROM coaching_consultations WHERE id=$1`, []any{id}, &state, &refunded)
		if state != "partially_refunded" || refunded != 200 {
			t.Fatalf("consultation %s %d", state, refunded)
		}
	}
	// floor(350 × 200 ÷ 500) = 140
	if got := scalar(t, `SELECT COALESCE(sum(a.amount_yen),0) FROM coach_refund_adjustments a JOIN coach_earnings e ON e.id=a.earning_id WHERE e.consultation_id=$1`, answered); got != 140 {
		t.Fatalf("consultation offset %d", got)
	}
	if reviewPaymentStatus(t, x.request) != "captured" {
		t.Fatal("unrelated payment changed")
	}
	// 回答時の報酬の記録（RecordConsultationEarnings）は一部返金の追加質問でも行い、その時点の累計で減らす。
	execSQLArgs(t, `UPDATE coaching_consultations SET status='answered',answered_at=now() WHERE id=$1`, unanswered)
	conn, err := pgx.Connect(context.Background(), testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	if err := db.New(conn).RecordConsultationEarnings(context.Background(), unanswered); err != nil {
		t.Fatal(err)
	}
	if got := scalar(t, `SELECT COALESCE(sum(a.amount_yen),0) FROM coach_refund_adjustments a JOIN coach_earnings e ON e.id=a.earning_id WHERE e.consultation_id=$1`, unanswered); got != 140 {
		t.Fatalf("later consultation earning offset %d", got)
	}
	if titledNotices(t, x.h, x.user, "追加質問の代金の一部を返金しました") != 2 {
		t.Fatal("consultation partial notice")
	}
}

// 返却後にまとめて記録する添削の報酬も、先に届いた部分返金の累計で減らす（報酬の記録は partially_refunded も対象）。
func TestCapturedPartialRefundBeforeReviewEarning(t *testing.T) {
	x := newCapturedFixture(t)
	execSQLArgs(t, `DELETE FROM review_payment_earning_links WHERE payment_id=$1`, x.payment)
	execSQLArgs(t, `DELETE FROM coach_earnings WHERE coach_id=$1`, x.coach)
	x.dashboard(t, "refund.created", "re_early", 500, "succeeded")
	if reviewPaymentStatus(t, x.request) != "partially_refunded" || netOffset(t) != 0 {
		t.Fatal("no earning yet")
	}
	x.run(t) // 送金の実行は、先に返却済みの添削の報酬を記録する。
	if scalar(t, `SELECT count(*) FROM coach_earnings WHERE review_request_id=$1`, x.request) != 1 {
		t.Fatal("partially refunded review earning not recorded")
	}
	var amount int
	queryRowArgs(t, `SELECT amount_yen FROM coach_earnings WHERE review_request_id=$1`, []any{x.request}, &amount)
	// 報酬は floor(1500 × 70%)=1050 のまま記録し、按分の 350 を相殺台帳で減らす。
	if want := amount * 500 / 1500; netOffset(t) != want {
		t.Fatalf("offset %d, want %d", netOffset(t), want)
	}
}

// T6：partially_refunded の支払いに管理者が返金すると、残額だけを Stripe に返金し、refunded に収束する。
func TestCapturedPartialRefundAdminRemainder(t *testing.T) {
	x := newCapturedFixture(t)
	x.paid(t)
	x.dashboard(t, "refund.created", "re_dash", 400, "succeeded")
	x.refund(t)
	refunds := x.f.refundList()
	if len(refunds) != 1 || refunds[0].Get("amount") != "1100" {
		t.Fatalf("remainder refund %v", refunds)
	}
	if reviewPaymentStatus(t, x.request) != "refunded" || x.refundedYen(t) != 1500 || netOffset(t) != 1050 {
		t.Fatalf("remainder state %s %d %d", reviewPaymentStatus(t, x.request), x.refundedYen(t), netOffset(t))
	}
	x.refund(t)
	if len(x.f.refundList()) != 1 {
		t.Fatal("remainder refunded twice")
	}
	// 管理者の残額の返金の通知（Stripe からの知らせ）が来ても変わらない。
	x.dashboard(t, "refund.updated", "re_1", 1100, "succeeded")
	if x.refundedYen(t) != 1500 || scalar(t, `SELECT count(*) FROM captured_payment_refunds`) != 2 || netOffset(t) != 1050 {
		t.Fatal("admin remainder webhook")
	}
	rec := doJSON(t, x.h, http.MethodGet, "/v1/admin/coaching/payments", x.admin, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"refunded_yen":1500`) {
		t.Fatalf("admin list %s", rec.Body.String())
	}
}

func TestCapturedPartialRefundAdminListShowsPartial(t *testing.T) {
	x := newCapturedFixture(t)
	x.dashboard(t, "refund.created", "re_dash", 400, "succeeded")
	rec := doJSON(t, x.h, http.MethodGet, "/v1/admin/coaching/payments", x.admin, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"status":"partially_refunded"`) || !strings.Contains(rec.Body.String(), `"refunded_yen":400`) {
		t.Fatalf("admin list %s", rec.Body.String())
	}
}

// T7：Webhook を経ずに Stripe にだけある全額返金・部分返金を、日次照合が同期し、検知を立てない。
// 同期できない返金（未知の PaymentIntent）は検知に残る。
func TestCapturedRefundDailyReconcileSelfHeals(t *testing.T) {
	x := newCapturedFixtureConfig(t, func(c *config.Config) { c.SecurityCheckSecret = "check-secret" })
	other := newCapturedFixtureInto(t, x)
	created := time.Now().Add(-5 * 24 * time.Hour).Unix()
	x.f.mu.Lock()
	x.f.listedRefunds = []map[string]any{
		{"id": "re_missed_partial", "amount": 500, "currency": "jpy", "status": "succeeded", "payment_intent": x.intent, "created": created},
		{"id": "re_missed_full", "amount": 1500, "currency": "jpy", "status": "succeeded", "payment_intent": other.intent, "created": created},
		{"id": "re_missed_unknown", "amount": 700, "currency": "jpy", "status": "succeeded", "payment_intent": "pi_unknown_" + uuid.NewString(), "created": created},
	}
	x.f.mu.Unlock()
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	req := doRawSecurityCheck(t, x.h, "check-secret")
	if req != http.StatusOK {
		t.Fatalf("security check %d", req)
	}
	if reviewPaymentStatus(t, x.request) != "partially_refunded" || x.refundedYen(t) != 500 {
		t.Fatal("missed partial refund not synchronized")
	}
	if reviewPaymentStatus(t, other.request) != "refunded" {
		t.Fatal("missed full refund not synchronized")
	}
	// 照合の幅は8日（Webhook の再送が尽きた後の返金も拾う）。
	x.f.mu.Lock()
	queries := append([]url.Values(nil), x.f.refundListQueries...)
	x.f.mu.Unlock()
	if len(queries) != 1 {
		t.Fatalf("refund list calls %d", len(queries))
	}
	since, _ := strconv.ParseInt(queries[0].Get("created[gte]"), 10, 64)
	if window := time.Since(time.Unix(since, 0)); window < 8*24*time.Hour-time.Minute || window > 8*24*time.Hour+time.Minute {
		t.Fatalf("reconcile window %s", window)
	}
	out := logs.String()
	if strings.Contains(out, "re_missed_partial") || strings.Contains(out, "re_missed_full") {
		t.Fatalf("synchronized refund alerted: %s", out)
	}
	if !strings.Contains(out, `"msg":"unknown stripe refund"`) || !strings.Contains(out, "re_missed_unknown") {
		t.Fatalf("unknown refund must stay alerted: %s", out)
	}
}

// T8：未成年の支払いの部分返金は、集計で件数1・金額は refunded_yen。全額返金は従来どおり全額。
func TestCapturedPartialRefundMinorSummary(t *testing.T) {
	x := newCapturedFixture(t)
	other := newCapturedFixtureInto(t, x)
	execSQLArgs(t, `UPDATE review_payments SET minor=true WHERE id IN ($1,$2)`, x.payment, other.payment)
	summary := func() (int, int) {
		t.Helper()
		var data struct {
			MinorRefunds struct {
				Total struct {
					Count  int `json:"review_count"`
					Amount int `json:"review_amount_yen"`
				} `json:"total"`
			} `json:"minor_refunds"`
		}
		rec := doJSON(t, x.h, http.MethodGet, "/v1/admin/coaching/payments", x.admin, nil)
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &data) != nil {
			t.Fatal(rec.Body.String())
		}
		return data.MinorRefunds.Total.Count, data.MinorRefunds.Total.Amount
	}
	x.dashboard(t, "refund.created", "re_minor_part", 300, "succeeded")
	if n, yen := summary(); n != 1 || yen != 300 {
		t.Fatalf("partial minor summary %d %d", n, yen)
	}
	rec := stripeWebhook(t, x.h, testWebhookSecret, "refund.created", map[string]any{"id": "re_minor_full", "currency": "jpy", "payment_intent": other.intent, "amount": 1500, "status": "succeeded"})
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	if n, yen := summary(); n != 2 || yen != 1800 {
		t.Fatalf("minor summary %d %d", n, yen)
	}
}

// 対象外の返金（jpy 以外・報酬との対応が欠けた支払い）は同期せず、支払いの状態を変えない。
func TestCapturedPartialRefundUnsupportedStaysUnsynchronized(t *testing.T) {
	x := newCapturedFixture(t)
	rec := stripeWebhook(t, x.h, testWebhookSecret, "refund.created", map[string]any{"id": "re_usd", "currency": "usd", "payment_intent": x.intent, "amount": 300, "status": "succeeded"})
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	if reviewPaymentStatus(t, x.request) != "captured" || scalar(t, `SELECT count(*) FROM captured_payment_refunds`) != 0 {
		t.Fatal("non-jpy refund synchronized")
	}
	// 支払額を超える同期はしない。
	x.dashboard(t, "refund.created", "re_big", 1000, "succeeded")
	x.dashboard(t, "refund.created", "re_over", 600, "succeeded")
	if x.refundedYen(t) != 1000 || scalar(t, `SELECT count(*) FROM captured_payment_refunds`) != 1 {
		t.Fatal("over-refund synchronized")
	}
}
