package httpx_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

// lastMonthJST は、日本時間の前の月の2日。
const lastMonthJST = `(date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo') - interval '1 month' + interval '1 day') AT TIME ZONE 'Asia/Tokyo'`

// The last minute of the previous month in Japan, independent of the DB session time zone.
const previousMonthEndJST = `(date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo') - interval '1 minute') AT TIME ZONE 'Asia/Tokyo'`

type earningsView struct {
	Payments       bool    `json:"payments"`
	StripeLinked   bool    `json:"stripe_linked"`
	NextPayoutDate *string `json:"next_payout_date"`
	NextPayoutYen  *int    `json:"next_payout_yen"`
	PayoutsEnabled bool    `json:"payouts_enabled"`
	ThisMonthYen   int     `json:"this_month_yen"`
	UnpaidYen      int     `json:"unpaid_yen"`
	Payouts        []struct {
		Month     string `json:"month"`
		AmountYen int    `json:"amount_yen"`
		Status    string `json:"status"`
	} `json:"payouts"`
	Earnings []struct {
		Source       string    `json:"source"`
		GrossYen     int       `json:"gross_yen"`
		SharePercent int       `json:"share_percent"`
		AmountYen    int       `json:"amount_yen"`
		Paid         bool      `json:"paid"`
		ConfirmedAt  time.Time `json:"confirmed_at"`
		CreatedAt    time.Time `json:"created_at"`
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

func TestCoachOnboardingRetriesAccountCreation(t *testing.T) {
	const path = "/v1/coach/stripe/onboarding"
	const link = "https://connect.stripe.test/acct_1"
	for _, tc := range []struct {
		name     string
		rejected bool
		expired  bool
	}{
		{name: "cached rejection gets a new key", rejected: true},
		{name: "lost response reuses the created account"},
		{name: "expired unresolved attempt stops sending requests", expired: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var keys []string
			rejectedKey := ""
			accounts := map[string]string{}
			blockResponse := true
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.ParseForm()
				mu.Lock()
				defer mu.Unlock()
				switch r.URL.Path {
				case "/v1/accounts":
					key := r.Header.Get("Idempotency-Key")
					keys = append(keys, key)
					if tc.rejected && blockResponse {
						rejectedKey = key
					}
					if tc.rejected && key == rejectedKey {
						w.WriteHeader(http.StatusBadRequest)
						fmt.Fprint(w, `{"error":{"type":"invalid_request_error","message":"platform profile incomplete"}}`)
						return
					}
					if _, exists := accounts[key]; !exists {
						accounts[key] = fmt.Sprintf("acct_%d", len(accounts)+1)
					}
					if blockResponse {
						// Stripe が作成したあと、レスポンスがこちらに届く前に切断する。
						conn, _, err := http.NewResponseController(w).Hijack()
						if err != nil {
							t.Error(err)
							return
						}
						_ = conn.Close()
						return
					}
					_ = json.NewEncoder(w).Encode(map[string]string{"id": accounts[key]})
				case "/v1/account_links":
					_ = json.NewEncoder(w).Encode(map[string]string{"url": "https://connect.stripe.test/" + r.PostForm.Get("account")})
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			t.Cleanup(srv.Close)
			h := setupConfig(t, func(cfg *config.Config) {
				cfg.StripeSecretKey = "sk_test_x"
				cfg.StripeAPIBase = srv.URL
				cfg.WebBaseURL = "https://web.test"
			})
			id, token, _ := planCoach(t, h, 3)
			execSQLArgs(t, `UPDATE coach_profiles SET stripe_account_id = NULL, payouts_enabled = false WHERE profile_id = $1`, id)
			if rec := doJSON(t, h, http.MethodPost, path, token, nil); rec.Code != http.StatusBadGateway {
				t.Fatalf("first attempt %d %s", rec.Code, rec.Body.String())
			}

			mu.Lock()
			blockResponse = false
			firstAttemptCalls := len(keys)
			mu.Unlock()
			if tc.expired {
				execSQLArgs(t, `UPDATE coach_stripe_account_attempts SET started_at = now() - interval '24 hours' WHERE profile_id = $1`, id)
				rec := doJSON(t, h, http.MethodPost, path, token, nil)
				if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "stripe_account_pending") {
					t.Fatalf("expired attempt %d %s", rec.Code, rec.Body.String())
				}
			} else {
				// 同時に再試行しても、片方だけが Stripe に口座作成を送る。
				retries := make(chan *httptest.ResponseRecorder, 2)
				for range 2 {
					go func() {
						retries <- doJSON(t, h, http.MethodPost, path, token, nil)
					}()
				}
				for range 2 {
					if rec := <-retries; rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), link) {
						t.Fatalf("retry %d %s", rec.Code, rec.Body.String())
					}
				}
			}

			mu.Lock()
			defer mu.Unlock()
			if firstAttemptCalls == 0 || len(accounts) != 1 || keys[0] != "account-"+id.String() {
				t.Fatalf("account creations %v, keys %v", accounts, keys)
			}
			for _, key := range keys[:firstAttemptCalls] {
				if key != keys[0] {
					t.Fatalf("automatic retries used different keys: %v", keys)
				}
			}
			if tc.expired {
				if len(keys) != firstAttemptCalls {
					t.Fatalf("sent another create after expiry: %v", keys)
				}
				return
			}
			if len(keys) != firstAttemptCalls+1 || keys[firstAttemptCalls] == "" ||
				(keys[firstAttemptCalls] != keys[0]) != tc.rejected {
				t.Fatalf("unexpected keys after retry and link refresh: %v", keys)
			}
		})
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
	offers := coachRequests(t, h, trc).Reviewing
	if len(offers) != 1 {
		t.Fatalf("offers %+v", offers)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+offers[0].ID+"/start", trc, nil); rec.Code != http.StatusOK {
		t.Fatalf("accept offer %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+reqID+"/response", trc, responseBody()); rec.Code != http.StatusOK {
		t.Fatalf("response %d %s", rec.Code, rec.Body.String())
	}

	// パーソナル指導: 8,000円の期間が終わった。手数料は25%。
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
	if e := coachEarnings(t, h, trc); e.ThisMonthYen != 350 || e.UnpaidYen != 350 || len(e.Earnings) != 1 || e.Earnings[0].Source != "review" || e.Earnings[0].SharePercent != 70 {
		t.Fatalf("review earnings %+v", e)
	}
	if e := coachEarnings(t, h, tc); e.UnpaidYen != 6000 || len(e.Earnings) != 1 || e.Earnings[0].GrossYen != 8000 || e.Earnings[0].SharePercent != 75 {
		t.Fatalf("personal earnings %+v", e)
	}

	// 今月の分は、まだ送金しない。
	rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/payouts/run", tadmin, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"created":0`) || len(f.transferList()) != 0 {
		t.Fatalf("run this month %d %s", rec.Code, rec.Body.String())
	}

	// 前の月の分になったら送る。3,000円に届かない添削の350円は繰り越す。
	execSQLArgs(t, `UPDATE coach_earnings SET confirmed_at = `+lastMonthJST)
	f.failNextTransfers(1)
	rec = doJSON(t, h, http.MethodPost, "/v1/admin/coaching/payouts/run", tadmin, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"created":1,"paid":0,"failed":1`) {
		t.Fatalf("run failed %d %s", rec.Code, rec.Body.String())
	}
	if e := coachEarnings(t, h, trc); e.UnpaidYen != 350 || len(e.Payouts) != 0 || e.Earnings[0].Paid {
		t.Fatalf("carried over %+v", e)
	}
	// 1回目は Stripe が断り、次に押したときにやり直す。繰り越した分が3,000円に届いたら一緒に送る。
	execSQLArgs(t, `INSERT INTO coach_earnings (id, coach_id, source, gross_yen, share_percent, amount_yen, confirmed_at)
		SELECT gen_random_uuid(), coach_id, source, gross_yen, share_percent, amount_yen, confirmed_at FROM coach_earnings, generate_series(1, 9) WHERE coach_id = $1`, reviewCoach)
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
	if got["acct_"+reviewCoach.String()] != "3500" || got["acct_"+personalCoach] != "6000" || len(got) != 2 {
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
	if e := coachEarnings(t, h, tc); e.UnpaidYen != 0 || len(e.Payouts) != 1 || e.Payouts[0].Status != "paid" || e.Payouts[0].AmountYen != 6000 || !e.Earnings[0].Paid {
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
// 3,000円に届かない分は、回数に上限なく繰り越し、累積が届いた締切月の翌月に送る（累積型・#357）。
// 取引が終わった（承認済みでなくなった）コーチには、届かなくても残りを全額送る。
func TestCoachPayoutCarriesOverUntilAccumulated(t *testing.T) {
	h, f := setupStripe(t)
	tadmin := adminToken(t, h)
	coach, _ := onDutyCoach(t, h, tadmin)
	earn := func(when string) {
		execSQLArgs(t, `INSERT INTO coach_earnings (id, coach_id, source, gross_yen, share_percent, amount_yen, confirmed_at)
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
	// 繰り越した分（前の前の月・さらに前の月の報酬）が残っていても、累積が届かなければ繰り越しを続ける。
	earn(lastMonthJST + ` - interval '1 month'`)
	if out := run(); !strings.Contains(out, `"created":0`) || len(f.transferList()) != 0 {
		t.Fatalf("carry over twice %s", out)
	}
	earn(lastMonthJST + ` - interval '2 month'`)
	if out := run(); !strings.Contains(out, `"created":0`) || len(f.transferList()) != 0 {
		t.Fatalf("carry over three times %s", out)
	}
	// 今月の分は締切前なので、累積に入れない。
	earn("now()")
	if out := run(); !strings.Contains(out, `"created":0`) || len(f.transferList()) != 0 {
		t.Fatalf("open month counted %s", out)
	}
	// 締切済みの累積が3,000円に届いたら、まとめて送る（840円×4＝3,360円）。
	earn(lastMonthJST)
	if out := run(); !strings.Contains(out, `"created":1,"paid":1`) {
		t.Fatalf("pay after accumulated %s", out)
	}
	if got := f.transferList(); len(got) != 1 || got[0].Get("amount") != "3360" {
		t.Fatalf("transfers %v", got)
	}

	// 取引が終わったコーチには、累積が届かなくても残りを全額送る。
	ended, _ := onDutyCoach(t, h, tadmin)
	execSQLArgs(t, `INSERT INTO coach_earnings (id, coach_id, source, gross_yen, share_percent, amount_yen, confirmed_at)
		VALUES (gen_random_uuid(), $1, 'review', 1200, 70, 840, `+lastMonthJST+`)`, ended)
	if out := run(); !strings.Contains(out, `"created":0`) || len(f.transferList()) != 1 {
		t.Fatalf("approved coach below minimum %s", out)
	}
	execSQLArgs(t, `UPDATE coach_profiles SET status='ended', on_duty=false WHERE profile_id=$1`, ended)
	if out := run(); !strings.Contains(out, `"created":1,"paid":1`) {
		t.Fatalf("final payout for ended coach %s", out)
	}
	if got := f.transferList(); len(got) != 2 || got[1].Get("destination") != "acct_"+ended.String() || got[1].Get("amount") != "840" {
		t.Fatalf("transfers %v", got)
	}
}

func TestCoachPayoutUsesConfirmationMonth(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	admin := adminToken(t, h)
	_, coach := onDutyCoach(t, h, admin)
	_, owner := demoRequester(t, h, admin)
	for range 3 {
		id := paidReview(t, h, owner, "card", "pi_month_boundary_"+uuid.NewString())
		offers := coachRequests(t, h, coach).Reviewing
		if len(offers) != 1 {
			t.Fatalf("offers %+v", offers)
		}
		if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+offers[0].ID+"/start", coach, nil); rec.Code != http.StatusOK {
			t.Fatalf("accept: %d %s", rec.Code, rec.Body.String())
		}
		if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", coach, responseBody()); rec.Code != http.StatusOK {
			t.Fatalf("response: %d %s", rec.Code, rec.Body.String())
		}
		execSQLArgs(t, `UPDATE review_requests SET returned_at = `+previousMonthEndJST+` WHERE id=$1`, id)
	}
	buyer, personalCoach, contract := adultAccepted(t, h)
	checkout(t, h, buyer, contract)
	paidSession(t, h, sessionIDFor(t, contract), "pi_personal_month_boundary", 8000)
	execSQLArgs(t, `UPDATE coaching_contracts SET ends_at = `+previousMonthEndJST+` WHERE id=$1`, contract)

	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"earnings_recorded":4`) {
		t.Fatalf("tick: %d %s", rec.Code, rec.Body.String())
	}
	for _, tc := range []struct {
		token string
		total int
	}{{coach, 1050}, {personalCoach, 6000}} {
		view := coachEarnings(t, h, tc.token)
		if view.ThisMonthYen != 0 || view.UnpaidYen != tc.total || len(view.Earnings) == 0 {
			t.Fatalf("earnings %+v", view)
		}
		for _, earning := range view.Earnings {
			jst := time.FixedZone("JST", 9*60*60)
			confirmed, created := earning.ConfirmedAt.In(jst), earning.CreatedAt.In(jst)
			if confirmed.IsZero() || created.IsZero() || confirmed.Hour() != 23 || confirmed.Minute() != 59 || !confirmed.Before(created) || confirmed.Month() == created.Month() {
				t.Fatalf("timestamps: confirmed %v, created %v", confirmed, created)
			}
		}
	}
	// 添削3本の1,050円（350円×3）は3,000円に届かないので繰り越し、サポートプランの分だけ送る。
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/payouts/run", admin, nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"created":1,"paid":1`) {
		t.Fatalf("run: %d %s", rec.Code, rec.Body.String())
	}
	if transfers := f.transferList(); len(transfers) != 1 {
		t.Fatalf("transfers: %d", len(transfers))
	}
}

func assertPayoutPreview(t *testing.T, view earningsView, amount int, deferred bool) {
	t.Helper()
	if amount == 0 {
		if view.NextPayoutDate != nil || view.NextPayoutYen != nil {
			t.Fatalf("expected null payout preview: %+v", view)
		}
		return
	}
	var date string
	queryRowArgs(t, `SELECT to_char(date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo') + interval '14 days' +
		CASE WHEN $1::boolean OR extract(day FROM now() AT TIME ZONE 'Asia/Tokyo') > 15
		THEN interval '1 month' ELSE interval '0' END, 'YYYY-MM-DD')`, []any{deferred}, &date)
	if view.NextPayoutDate == nil || *view.NextPayoutDate != date || view.NextPayoutYen == nil || *view.NextPayoutYen != amount {
		t.Fatalf("preview date=%v yen=%v, want %s / %d", view.NextPayoutDate, view.NextPayoutYen, date, amount)
	}
}

func TestCoachPayoutPreviewCases(t *testing.T) {
	for _, tc := range []struct {
		name, confirmedAt, status string
		amount, expected          int
		deferred, unlinked        bool
	}{
		{name: "A", confirmedAt: lastMonthJST, amount: 3150, expected: 3150},
		// 累積が3,000円に届いていない分は、いつ支払うかが決まらないので予定日を出さない（累積型・#357）。
		{name: "B", confirmedAt: lastMonthJST, amount: 1050},
		{name: "C", confirmedAt: lastMonthJST + ` - interval '1 month'`, amount: 1050},
		{name: "C2", confirmedAt: lastMonthJST + ` - interval '1 month'`, amount: 3150, expected: 3150},
		{name: "D", confirmedAt: previousMonthEndJST, amount: 3150, expected: 3150},
		{name: "open month", confirmedAt: "now()", amount: 3150},
		{name: "no account", confirmedAt: lastMonthJST, amount: 3150, unlinked: true},
		{name: "suspended", confirmedAt: lastMonthJST, amount: 3150, status: "suspended"},
		{name: "final settlement", confirmedAt: lastMonthJST, amount: 1050, expected: 1050, status: "ended"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := setupStripe(t)
			coach, tok := onDutyCoach(t, h, adminToken(t, h))
			execSQLArgs(t, `INSERT INTO coach_earnings (id, coach_id, source, gross_yen, share_percent, amount_yen, confirmed_at)
				VALUES (gen_random_uuid(), $1, 'review', $2, 70, $2, `+tc.confirmedAt+`)`, coach, tc.amount)
			if tc.unlinked {
				execSQLArgs(t, `UPDATE coach_profiles SET stripe_account_id=NULL, payouts_enabled=false WHERE profile_id=$1`, coach)
			}
			if tc.status != "" {
				execSQLArgs(t, `UPDATE coach_profiles SET status=$2, on_duty=false WHERE profile_id=$1`, coach, tc.status)
			}
			view := coachEarnings(t, h, tok)
			assertPayoutPreview(t, view, tc.expected, tc.deferred)
			if tc.name == "D" && view.ThisMonthYen != 0 {
				t.Fatalf("previous month's earning in current total: %d", view.ThisMonthYen)
			}
		})
	}
}
