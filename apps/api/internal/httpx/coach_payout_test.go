package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

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
	if len(f.accounts) != 1 || f.accounts[0].Get("type") != "express" || f.accounts[0].Get("country") != "JP" {
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

	// パーソナル指導: 初年10%のコーチの8,000円の期間が終わった。
	tu, tc, contractID := adultAccepted(t, h)
	var personalCoach string
	queryRowArgs(t, `SELECT coach_id FROM coaching_contracts WHERE id = $1`, []any{contractID}, &personalCoach)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/coaches/"+personalCoach+"/reduced-fee", tc, map[string]any{"enabled": true}); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin reduced fee %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/coaches/"+personalCoach+"/reduced-fee", tadmin, map[string]any{"enabled": true}); rec.Code != http.StatusOK {
		t.Fatalf("reduced fee %d %s", rec.Code, rec.Body.String())
	}
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
	if e := coachEarnings(t, h, trc); e.ThisMonthYen != 700 || e.UnpaidYen != 700 || len(e.Earnings) != 1 || e.Earnings[0].Source != "review" || e.Earnings[0].SharePercent != 70 {
		t.Fatalf("review earnings %+v", e)
	}
	if e := coachEarnings(t, h, tc); e.UnpaidYen != 7200 || len(e.Earnings) != 1 || e.Earnings[0].GrossYen != 8000 || e.Earnings[0].SharePercent != 90 {
		t.Fatalf("personal earnings %+v", e)
	}

	// 今月の分は、まだ送金しない。
	rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/payouts/run", tadmin, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"created":0`) || len(f.transferList()) != 0 {
		t.Fatalf("run this month %d %s", rec.Code, rec.Body.String())
	}

	// 前の月の分になったら送る。1回目は Stripe が断り、次に押したときにやり直す。
	execSQLArgs(t, `UPDATE coach_earnings SET created_at = created_at - interval '40 days'`)
	f.failNextTransfers(2)
	rec = doJSON(t, h, http.MethodPost, "/v1/admin/coaching/payouts/run", tadmin, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"created":2`) || !strings.Contains(rec.Body.String(), `"failed":2`) {
		t.Fatalf("run failed %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/admin/coaching/payouts/run", tadmin, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"created":0`) || !strings.Contains(rec.Body.String(), `"paid":2`) {
		t.Fatalf("run retry %d %s", rec.Code, rec.Body.String())
	}
	got := map[string]string{}
	for _, tr := range f.transferList() {
		if tr.Get("currency") != "jpy" {
			t.Fatalf("transfer %v", tr)
		}
		got[tr.Get("destination")] = tr.Get("amount")
	}
	if got["acct_"+reviewCoach.String()] != "700" || got["acct_"+personalCoach] != "7200" || len(got) != 2 {
		t.Fatalf("transfers %v", got)
	}
	// 済んだものは送り直さない。
	rec = doJSON(t, h, http.MethodPost, "/v1/admin/coaching/payouts/run", tadmin, nil)
	if !strings.Contains(rec.Body.String(), `"created":0,"paid":0,"failed":0`) || len(f.transferList()) != 2 {
		t.Fatalf("run again %s", rec.Body.String())
	}
	if e := coachEarnings(t, h, tc); e.UnpaidYen != 0 || len(e.Payouts) != 1 || e.Payouts[0].Status != "paid" || e.Payouts[0].AmountYen != 7200 || !e.Earnings[0].Paid {
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
