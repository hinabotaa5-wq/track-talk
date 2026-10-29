package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

// lastMonthJST は、日本時間の前の月の2日。
const lastMonthJST = `(date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo') - interval '1 month' + interval '1 day') AT TIME ZONE 'Asia/Tokyo'`

type earningsView struct {
	Payments       bool `json:"payments"`
	StripeLinked   bool `json:"stripe_linked"`
	PayoutsEnabled bool `json:"payouts_enabled"`
	ThisMonthYen   int  `json:"this_month_yen"`
	UnpaidYen      int  `json:"unpaid_yen"`
	Payouts        []struct {
		Month     string `json:"month"`
		AmountYen int    `json:"amount_yen"`
		Status    string `json:"status"`
	} `json:"payouts"`
	Earnings []struct {
		Source       string `json:"source"`
		GrossYen     int    `json:"gross_yen"`
		SharePercent int    `json:"share_percent"`
		AmountYen    int    `json:"amount_yen"`
		Paid         bool   `json:"paid"`
	} `json:"earnings"`
}

func coachEarnings(t *testing.T, h http.Handler, tc string) earningsView {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/coach/me/earnings", tc, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("earnings %d %s", rec.Code, rec.Body.String())
	}
	var out earningsView
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

func TestCoachOnboardingGatesDutyAndContracts(t *testing.T) {
	h, f := setupStripe(t)
	tadmin := adminToken(t, h)
	id := uuid.New()
	tc := token(t, id)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", tc, coachApplicationBody()); rec.Code != http.StatusOK {
		t.Fatalf("apply %d", rec.Code)
	}
	withCoachPhoto(t, id)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+id.String()+"/approve", tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("approve %d", rec.Code)
	}
	execSQLArgs(t, `INSERT INTO push_subscriptions (id, profile_id, endpoint, p256dh, auth) VALUES ($1, $2, $3, 'key', 'auth')`,
		uuid.New(), id, "https://push.example.com/"+uuid.NewString())

	// 口座の登録が済むまでは、受付を始められない。
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", tc, map[string]any{"on_duty": true}); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "payouts_required") {
		t.Fatalf("on duty before onboarding %d %s", rec.Code, rec.Body.String())
	}
	account := "acct_" + id.String()
	for range 2 {
		rec := doJSON(t, h, http.MethodPost, "/v1/coach/stripe/onboarding", tc, nil)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "https://connect.stripe.test/"+account) {
			t.Fatalf("onboarding %d %s", rec.Code, rec.Body.String())
		}
	}
	if len(f.accounts) != 1 || f.accounts[0].Get("type") != "express" || f.accounts[0].Get("country") != "JP" ||
		f.accounts[0].Get("settings[payouts][schedule][interval]") != "monthly" || f.accounts[0].Get("settings[payouts][schedule][monthly_anchor]") != "15" {
		t.Fatalf("accounts %v", f.accounts)
	}
	if e := coachEarnings(t, h, tc); !e.Payments || !e.StripeLinked || e.PayoutsEnabled {
		t.Fatalf("before enabled %+v", e)
	}
	// Stripe で登録が済んだら、画面を開いたときに読みにいく。
	f.enable(account)
	if e := coachEarnings(t, h, tc); !e.PayoutsEnabled {
		t.Fatalf("after enabled %+v", e)
	}
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", tc, map[string]any{"on_duty": true}); rec.Code != http.StatusOK {
		t.Fatalf("on duty %d %s", rec.Code, rec.Body.String())
	}

	// 承認前の利用者には使えない。
	_, tu := demoUser(t, h, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/stripe/onboarding", tu, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("not coach %d", rec.Code)
	}
}

func TestCoachAcceptNeedsPayouts(t *testing.T) {
	h, _ := setupStripe(t)
	coach, tc, planID := planCoach(t, h, 3)
	execSQLArgs(t, `UPDATE coach_profiles SET payouts_enabled = false WHERE profile_id = $1`, coach)
	_, tu := demoUser(t, h, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts", tu, contractBody(planID))
	if rec.Code != http.StatusCreated {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	id := decodeContract(t, rec.Body.Bytes()).ID
	rec = doJSON(t, h, http.MethodPost, "/v1/coach/contracts/"+id+"/accept", tc, map[string]any{"agree_report": true, "agree_detection": true})
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "payouts_required") {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}
}

func TestCoachEarningsAndMonthlyPayout(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	tadmin := adminToken(t, h)

	// 添削: 返却して確定した1,000円のうち7割。
	reviewCoach, trc := onDutyCoach(t, h, tadmin)
	_, tr := demoRequester(t, h, tadmin)
	reqID := paidReview(t, h, tr, "card", "pi_card_earn")
	offers := coachRequests(t, h, trc).Offers
	if len(offers) != 1 {
		t.Fatalf("offers %+v", offers)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/offers/"+offers[0].ID+"/accept", trc, nil); rec.Code != http.StatusOK {
		t.Fatalf("accept offer %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+reqID+"/response", trc, responseBody()); rec.Code != http.StatusOK {
		t.Fatalf("response %d %s", rec.Code, rec.Body.String())
	}

	// パーソナル指導: 8,000円の期間が終わった。手数料は20%。
	tu, tc, contractID := adultAccepted(t, h)
	var personalCoach string
	queryRowArgs(t, `SELECT coach_id FROM coaching_contracts WHERE id = $1`, []any{contractID}, &personalCoach)
	checkout(t, h, tu, contractID)
	paidSession(t, h, sessionIDFor(t, contractID), "pi_personal_earn", 8000)
	if contractStatus(t, contractID) != "active" {
		t.Fatalf("status %s", contractStatus(t, contractID))
	}
	execSQLArgs(t, `UPDATE coaching_contracts SET ends_at = now() - interval '1 minute' WHERE id = $1`, contractID)
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"earnings_recorded":2`) {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
	// 2回目は増えない。
	if rec := tick(t, h, "tick-secret"); !strings.Contains(rec.Body.String(), `"earnings_recorded":0`) {
		t.Fatalf("tick again %s", rec.Body.String())
	}
	if e := coachEarnings(t, h, trc); e.ThisMonthYen != 1050 || e.UnpaidYen != 1050 || len(e.Earnings) != 1 || e.Earnings[0].Source != "review" || e.Earnings[0].SharePercent != 70 {
		t.Fatalf("review earnings %+v", e)
	}
	if e := coachEarnings(t, h, tc); e.UnpaidYen != 6400 || len(e.Earnings) != 1 || e.Earnings[0].GrossYen != 8000 || e.Earnings[0].SharePercent != 80 {
		t.Fatalf("personal earnings %+v", e)
	}

	// 今月の分は、まだ送金しない。
	rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/payouts/run", tadmin, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"created":0`) || len(f.transferList()) != 0 {
		t.Fatalf("run this month %d %s", rec.Code, rec.Body.String())
	}

	// 前の月の分になったら送る。3,000円に届かない添削の1,050円は繰り越す。
	execSQLArgs(t, `UPDATE coach_earnings SET created_at = `+lastMonthJST)
	f.failNextTransfers(1)
	rec = doJSON(t, h, http.MethodPost, "/v1/admin/coaching/payouts/run", tadmin, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"created":1,"paid":0,"failed":1`) {
		t.Fatalf("run failed %d %s", rec.Code, rec.Body.String())
	}
	if e := coachEarnings(t, h, trc); e.UnpaidYen != 1050 || len(e.Payouts) != 0 || e.Earnings[0].Paid {
		t.Fatalf("carried over %+v", e)
	}
	// 1回目は Stripe が断り、次に押したときにやり直す。繰り越した分が3,000円に届いたら一緒に送る。
	execSQLArgs(t, `INSERT INTO coach_earnings (id, coach_id, source, gross_yen, share_percent, amount_yen, created_at)
		SELECT gen_random_uuid(), coach_id, source, gross_yen, share_percent, amount_yen, created_at FROM coach_earnings, generate_series(1, 3) WHERE coach_id = $1`, reviewCoach)
	rec = doJSON(t, h, http.MethodPost, "/v1/admin/coaching/payouts/run", tadmin, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"created":1`) || !strings.Contains(rec.Body.String(), `"paid":2`) {
		t.Fatalf("run retry %d %s", rec.Code, rec.Body.String())
	}
	got := map[string]string{}
	for _, tr := range f.transferList() {
		if tr.Get("currency") != "jpy" {
			t.Fatalf("transfer %v", tr)
		}
		got[tr.Get("destination")] = tr.Get("amount")
	}
	if got["acct_"+reviewCoach.String()] != "4200" || got["acct_"+personalCoach] != "6400" || len(got) != 2 {
		t.Fatalf("transfers %v", got)
	}
	// 済んだものは送り直さない。
	rec = doJSON(t, h, http.MethodPost, "/v1/admin/coaching/payouts/run", tadmin, nil)
	if !strings.Contains(rec.Body.String(), `"created":0,"paid":0,"failed":0`) || len(f.transferList()) != 2 {
		t.Fatalf("run again %s", rec.Body.String())
	}
	// Stripe では送れていたのに、こちらで失敗のまま残った送金は、送り直さずに済ませる。
	execSQLArgs(t, `UPDATE coach_payouts SET status = 'failed', stripe_transfer_id = NULL, paid_at = NULL`)
	rec = doJSON(t, h, http.MethodPost, "/v1/admin/coaching/payouts/run", tadmin, nil)
	if !strings.Contains(rec.Body.String(), `"created":0,"paid":2,"failed":0`) || len(f.transferList()) != 2 {
		t.Fatalf("run after lost update %s %d", rec.Body.String(), len(f.transferList()))
	}
	if e := coachEarnings(t, h, tc); e.UnpaidYen != 0 || len(e.Payouts) != 1 || e.Payouts[0].Status != "paid" || e.Payouts[0].AmountYen != 6400 || !e.Earnings[0].Paid {
		t.Fatalf("after payout %+v", e)
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/admin/coaching/payouts", tadmin, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"paid"`) || !strings.Contains(rec.Body.String(), `"payouts_enabled":true`) {
		t.Fatalf("admin payouts %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/admin/coaching/payouts", tc, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin payouts %d", rec.Code)
	}
}

func TestCoachPayoutsDemoMode(t *testing.T) {
	h := setup(t)
	_, tc, _ := planCoach(t, h, 3)
	if e := coachEarnings(t, h, tc); e.Payments {
		t.Fatalf("demo %+v", e)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/stripe/onboarding", tc, nil); rec.Code != http.StatusConflict {
		t.Fatalf("onboarding demo %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/payouts/run", adminToken(t, h), nil); rec.Code != http.StatusConflict {
		t.Fatalf("run demo %d", rec.Code)
	}
}

func TestStripeWebhookSyncsCoachAccountAndAlertsDispute(t *testing.T) {
	h, f := setupStripe(t)
	tadmin := adminToken(t, h)
	id := uuid.New()
	tc := token(t, id)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", tc, coachApplicationBody()); rec.Code != http.StatusOK {
		t.Fatalf("apply %d", rec.Code)
	}
	withCoachPhoto(t, id)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+id.String()+"/approve", tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("approve %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/stripe/onboarding", tc, nil); rec.Code != http.StatusOK {
		t.Fatalf("onboarding %d", rec.Code)
	}
	account := "acct_" + id.String()
	enabled := func() bool {
		var ok bool
		queryRowArgs(t, `SELECT payouts_enabled FROM coach_profiles WHERE profile_id = $1`, []any{id}, &ok)
		return ok
	}

	// Connect の知らせは、別の鍵で署名されて届く。
	update := func(secret string, payouts bool) int {
		return stripeWebhook(t, h, secret, "account.updated", map[string]any{"id": account, "payouts_enabled": payouts}).Code
	}
	f.setEnabled(account, true)
	if code := update(testConnectWebhookSecret, true); code != http.StatusOK || !enabled() {
		t.Fatalf("enable %d %v", code, enabled())
	}
	// 知らせは順番どおりに届くとは限らない。古い「受け取れない」が後から届いても、いまの状態を読んで決める。
	if code := update(testConnectWebhookSecret, false); code != http.StatusOK || !enabled() {
		t.Fatalf("stale disable %d %v", code, enabled())
	}
	// あとから受け取れなくなったら、送金の対象から外す。
	f.setEnabled(account, false)
	if code := update(testConnectWebhookSecret, false); code != http.StatusOK || enabled() {
		t.Fatalf("disable %d %v", code, enabled())
	}
	if code := update("whsec_other", true); code != http.StatusBadRequest || enabled() {
		t.Fatalf("bad signature %d %v", code, enabled())
	}
	// 知らないアカウントの知らせは読み飛ばす。
	if rec := stripeWebhook(t, h, testConnectWebhookSecret, "account.updated", map[string]any{"id": "acct_unknown", "payouts_enabled": true}); rec.Code != http.StatusOK {
		t.Fatalf("unknown account %d", rec.Code)
	}
	if rec := stripeWebhook(t, h, testWebhookSecret, "charge.dispute.created", map[string]any{"id": "dp_1", "amount": 1000, "reason": "fraudulent", "payment_intent": "pi_x"}); rec.Code != http.StatusOK {
		t.Fatalf("dispute %d", rec.Code)
	}
}

// コーチングを閉じても、決済への通知は署名を確かめて受け付ける。
func TestStripeWebhookAcceptsDisputeWhileCoachingIsOff(t *testing.T) {
	h := setupConfig(t, func(cfg *config.Config) {
		cfg.CoachingEnabled = false
		cfg.StripeSecretKey = "sk_test_x"
		cfg.StripeWebhookSecret = testWebhookSecret
	})
	if rec := stripeWebhook(t, h, testWebhookSecret, "charge.dispute.created", map[string]any{"id": "dp_1"}); rec.Code != http.StatusOK {
		t.Fatalf("webhook while off %d", rec.Code)
	}
}

// 返金で1円だけ残った契約は取り分が0円になる。記録しないで、tick 全体を止めない。
func TestCoachEarningsSkipZeroShare(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	tu, _, contractID := adultAccepted(t, h)
	checkout(t, h, tu, contractID)
	paidSession(t, h, sessionIDFor(t, contractID), "pi_personal_one_yen", 8000)
	execSQLArgs(t, `UPDATE coaching_payments SET status = 'partially_refunded', refunded_yen = amount_yen - 1 WHERE contract_id = $1`, contractID)
	execSQLArgs(t, `UPDATE coaching_contracts SET status = 'refunded' WHERE id = $1`, contractID)
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"earnings_recorded":0`) {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
}

// 期間が終わってから全額を返金したら、まだ送金していない報酬を外す。送金に含めたあとは返金させない。
func TestFullRefundAfterEndRemovesEarning(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	tadmin := adminToken(t, h)
	tu, tc, contractID := adultAccepted(t, h)
	checkout(t, h, tu, contractID)
	paidSession(t, h, sessionIDFor(t, contractID), "pi_personal_refund_after_end", 8000)
	execSQLArgs(t, `UPDATE coaching_contracts SET ends_at = now() - interval '1 minute' WHERE id = $1`, contractID)
	if rec := tick(t, h, "tick-secret"); !strings.Contains(rec.Body.String(), `"earnings_recorded":1`) {
		t.Fatalf("tick %s", rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/contracts/"+contractID+"/refund", tadmin, map[string]any{"kind": "full"}); rec.Code != http.StatusOK {
		t.Fatalf("refund %d %s", rec.Code, rec.Body.String())
	}
	if e := coachEarnings(t, h, tc); e.UnpaidYen != 0 || len(e.Earnings) != 0 {
		t.Fatalf("earnings after refund %+v", e)
	}
	if rec := tick(t, h, "tick-secret"); !strings.Contains(rec.Body.String(), `"earnings_recorded":0`) {
		t.Fatalf("tick after refund %s", rec.Body.String())
	}

	// 送金に含めた報酬がある指導は、ここからは返金できない。
	tu2, _, other := adultAccepted(t, h)
	checkout(t, h, tu2, other)
	paidSession(t, h, sessionIDFor(t, other), "pi_personal_paid_out", 8000)
	execSQLArgs(t, `UPDATE coaching_contracts SET ends_at = now() - interval '1 minute' WHERE id = $1`, other)
	tick(t, h, "tick-secret")
	execSQLArgs(t, `INSERT INTO coach_payouts (id, coach_id, month, amount_yen) SELECT gen_random_uuid(), coach_id, date_trunc('month', now())::date, amount_yen FROM coach_earnings WHERE contract_id = $1`, other)
	execSQLArgs(t, `UPDATE coach_earnings e SET payout_id = p.id FROM coach_payouts p WHERE p.coach_id = e.coach_id AND e.contract_id = $1`, other)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/contracts/"+other+"/refund", tadmin, map[string]any{"kind": "full"}); rec.Code != http.StatusConflict {
		t.Fatalf("refund after payout %d %s", rec.Code, rec.Body.String())
	}
}

// 3,000円に届かない報酬の繰り越しは1回まで。前の前の月の分が残っていれば、届かなくても送る。
func TestCoachPayoutCarriesOverOnce(t *testing.T) {
	h, f := setupStripe(t)
	tadmin := adminToken(t, h)
	coach, _ := onDutyCoach(t, h, tadmin)
	earn := func(when string) {
		execSQLArgs(t, `INSERT INTO coach_earnings (id, coach_id, source, gross_yen, share_percent, amount_yen, created_at)
			VALUES (gen_random_uuid(), $1, 'review', 1200, 70, 840, `+when+`)`, coach)
	}
	run := func() string {
		rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/payouts/run", tadmin, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("run %d %s", rec.Code, rec.Body.String())
		}
		return rec.Body.String()
	}
	earn(lastMonthJST)
	if out := run(); !strings.Contains(out, `"created":0`) || len(f.transferList()) != 0 {
		t.Fatalf("carry over %s", out)
	}
	// 前の月に繰り越した分（ここでは前の前の月の報酬）が残っていれば、合計が届かなくても送る。
	earn(lastMonthJST + ` - interval '1 month'`)
	if out := run(); !strings.Contains(out, `"created":1,"paid":1`) {
		t.Fatalf("pay after one carry over %s", out)
	}
	if got := f.transferList(); len(got) != 1 || got[0].Get("amount") != "1680" {
		t.Fatalf("transfers %v", got)
	}
}
