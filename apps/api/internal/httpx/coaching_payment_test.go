package httpx_test

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/stripe"
)

const (
	testWebhookSecret        = "whsec_test"
	testConnectWebhookSecret = "whsec_connect_test"
)

// fakeStripe は、Stripe の代わりに支払い画面と返金を受け付けるサーバー。
type fakeStripe struct {
	mu       sync.Mutex
	sessions []url.Values
	refunds  []url.Values
	// 仮押さえの確定・取り消しを受けた PaymentIntent。
	captured []string
	canceled []string
	// Connect: 作ったアカウント、受け取りの準備ができたアカウント、送金。
	accounts  []url.Values
	enabled   map[string]bool
	transfers []url.Values
	// 送金を断らせるときの数。
	failTransfers int
}

func (f *fakeStripe) transferList() []url.Values {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]url.Values(nil), f.transfers...)
}

func (f *fakeStripe) enable(account string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.enabled[account] = true
}

func (f *fakeStripe) failNextTransfers(n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failTransfers = n
}

func (f *fakeStripe) calls() (captured, canceled []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.captured...), append([]string(nil), f.canceled...)
}

func (f *fakeStripe) lastSession() url.Values {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sessions[len(f.sessions)-1]
}

func (f *fakeStripe) refundList() []url.Values {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]url.Values(nil), f.refunds...)
}

func setupStripe(t *testing.T) (http.Handler, *fakeStripe) {
	t.Helper()
	f := &fakeStripe{enabled: map[string]bool{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, _, _ := r.BasicAuth(); user != "sk_test_x" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = r.ParseForm()
		f.mu.Lock()
		defer f.mu.Unlock()
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/payment_intents/"):
			// テストでは、pi_card で始まるものを仮押さえ済み、それ以外を確定済みとして答える。
			id := strings.TrimPrefix(r.URL.Path, "/v1/payment_intents/")
			status := "succeeded"
			if strings.HasPrefix(id, "pi_card") {
				status = "requires_capture"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "status": status})
		case strings.HasSuffix(r.URL.Path, "/capture"):
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/payment_intents/"), "/capture")
			f.captured = append(f.captured, id)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "status": "succeeded"})
		case strings.HasSuffix(r.URL.Path, "/cancel"):
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/payment_intents/"), "/cancel")
			f.canceled = append(f.canceled, id)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "status": "canceled"})
		case r.URL.Path == "/v1/checkout/sessions":
			f.sessions = append(f.sessions, r.PostForm)
			n := len(f.sessions)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": fmt.Sprintf("cs_test_%d_%s", n, r.PostForm.Get("client_reference_id")), "url": fmt.Sprintf("https://checkout.stripe.test/%d", n),
				"expires_at": json.Number(r.PostForm.Get("expires_at")),
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/accounts":
			f.accounts = append(f.accounts, r.PostForm)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "acct_" + r.PostForm.Get("metadata[profile_id]")})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/accounts/"):
			id := strings.TrimPrefix(r.URL.Path, "/v1/accounts/")
			_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "payouts_enabled": f.enabled[id], "details_submitted": f.enabled[id]})
		case r.URL.Path == "/v1/account_links":
			_ = json.NewEncoder(w).Encode(map[string]any{"url": "https://connect.stripe.test/" + r.PostForm.Get("account")})
		case r.URL.Path == "/v1/transfers":
			if f.failTransfers > 0 {
				f.failTransfers--
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"type": "invalid_request_error", "code": "balance_insufficient", "message": "insufficient funds"}})
				return
			}
			f.transfers = append(f.transfers, r.PostForm)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": fmt.Sprintf("tr_%d", len(f.transfers)), "amount": json.Number(r.PostForm.Get("amount"))})
		case r.URL.Path == "/v1/refunds":
			f.refunds = append(f.refunds, r.PostForm)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": fmt.Sprintf("re_%d", len(f.refunds)), "status": "succeeded"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	h := setupConfig(t, func(cfg *config.Config) {
		cfg.StripeSecretKey = "sk_test_x"
		cfg.StripeWebhookSecret = testWebhookSecret
		cfg.StripeConnectWebhookSecret = testConnectWebhookSecret
		cfg.StripeAPIBase = srv.URL
		cfg.WebBaseURL = "https://web.test"
		cfg.CoachingTickSecret = "tick-secret"
	})
	return h, f
}

// sessionIDFor は、払う人が開いた支払い画面の ID を DB から読む。
func sessionIDFor(t *testing.T, contractID string) string {
	t.Helper()
	var id string
	queryRowArgs(t, `SELECT checkout_session_id FROM coaching_payments WHERE contract_id = $1 ORDER BY created_at DESC LIMIT 1`, []any{contractID}, &id)
	return id
}

func stripeWebhook(t *testing.T, h http.Handler, secret, typ string, object map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	payload, _ := json.Marshal(map[string]any{"id": "evt_" + fmt.Sprint(time.Now().UnixNano()), "type": typ, "data": map[string]any{"object": object}})
	ts := time.Now().Unix()
	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", strings.NewReader(string(payload)))
	req.Header.Set("Stripe-Signature", fmt.Sprintf("t=%d,v1=%s", ts, hex.EncodeToString(stripe.Sign(payload, secret, ts))))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func paidSession(t *testing.T, h http.Handler, session, intent string, amount int) {
	t.Helper()
	rec := stripeWebhook(t, h, testWebhookSecret, "checkout.session.completed", map[string]any{
		"id": session, "payment_status": "paid", "payment_intent": intent, "amount_total": amount,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("webhook %d %s", rec.Code, rec.Body.String())
	}
}

// adultAccepted は、大人の利用者が申し込み、コーチが承諾して支払い待ちになったところまで進める。
func adultAccepted(t *testing.T, h http.Handler) (tu, tc, id string) {
	t.Helper()
	_, tc, planID := planCoach(t, h, 3)
	_, tu = demoUser(t, h, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts", tu, contractBody(planID))
	if rec.Code != http.StatusCreated {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	id = decodeContract(t, rec.Body.Bytes()).ID
	rec = doJSON(t, h, http.MethodPost, "/v1/coach/contracts/"+id+"/accept", tc, map[string]any{"agree_report": true, "agree_detection": true})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"payment_required":true`) {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}
	if ct := decodeContract(t, rec.Body.Bytes()); ct.Status != "accepted" || ct.StartsAt != nil {
		t.Fatalf("accepted %+v", ct)
	}
	return tu, tc, id
}

func checkout(t *testing.T, h http.Handler, tok, id string) string {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/checkout", tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("checkout %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		URL string `json:"checkout_url"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out.URL
}

func contractStatus(t *testing.T, id string) string {
	t.Helper()
	var status string
	queryRowArgs(t, `SELECT status FROM coaching_contracts WHERE id = $1`, []any{id}, &status)
	return status
}

func TestCoachingPaymentStartsTheContract(t *testing.T) {
	h, f := setupStripe(t)
	tu, tc, id := adultAccepted(t, h)

	// 払うまでは始まらない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/messages", tc, map[string]any{"body": "よろしくお願いします"}); rec.Code != http.StatusConflict {
		t.Fatalf("send before paying %d", rec.Code)
	}
	_, other := demoUser(t, h, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/checkout", other, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("other checkout %d", rec.Code)
	}
	first := checkout(t, h, tu, id)
	if again := checkout(t, h, tu, id); again != first || len(f.sessions) != 1 {
		t.Fatalf("reuse %q %q %d", first, again, len(f.sessions))
	}
	form := f.lastSession()
	if form.Get("line_items[0][price_data][unit_amount]") != "8000" || form.Get("line_items[0][price_data][currency]") != "jpy" ||
		form.Get("mode") != "payment" || form.Get("metadata[contract_id]") != id ||
		form.Get("success_url") != "https://web.test/coaching/personal/"+id+"?paid=1" {
		t.Fatalf("session form %v", form)
	}

	session := sessionIDFor(t, id)
	if rec := stripeWebhook(t, h, "whsec_wrong", "checkout.session.completed", map[string]any{"id": session, "payment_status": "paid", "payment_intent": "pi_1"}); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad signature %d", rec.Code)
	}
	if contractStatus(t, id) != "accepted" {
		t.Fatal("started by a bad signature")
	}
	paidSession(t, h, session, "pi_1", 8000)
	paidSession(t, h, session, "pi_1", 8000) // 同じ知らせが2回届いても1回だけ。
	rec := doJSON(t, h, http.MethodGet, "/v1/coaching/contracts/"+id, tu, nil)
	if ct := decodeContract(t, rec.Body.Bytes()); ct.Status != "active" || ct.StartsAt == nil {
		t.Fatalf("started %+v", ct)
	}
	if !strings.Contains(rec.Body.String(), `"payment":{"amount_yen":8000,"paid_at":`) || !strings.Contains(rec.Body.String(), `"status":"paid"`) {
		t.Fatalf("payment %s", rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/checkout", tu, nil); rec.Code != http.StatusConflict {
		t.Fatalf("checkout after paying %d", rec.Code)
	}

	// 二重に払われたら、あとのほうを全額返金する。
	execSQLArgs(t, `INSERT INTO coaching_payments (id, contract_id, payer, amount_yen, checkout_session_id, checkout_url, session_expires_at)
		VALUES (gen_random_uuid(), $1, 'requester', 8000, 'cs_dup', 'https://checkout.stripe.test/dup', now() + interval '1 hour')`, id)
	paidSession(t, h, "cs_dup", "pi_dup", 8000)
	refunds := f.refundList()
	if len(refunds) != 1 || refunds[0].Get("payment_intent") != "pi_dup" || refunds[0].Get("amount") != "8000" {
		t.Fatalf("duplicate refund %v", refunds)
	}
	if contractStatus(t, id) != "active" {
		t.Fatal("duplicate payment changed the contract")
	}

	// 運営が残りの日数分を返金すると、その時点で終わる。30日のうち15日が残っていれば半分。
	execSQLArgs(t, `UPDATE coaching_contracts SET starts_at = now() - interval '15 days', ends_at = now() + interval '15 days' WHERE id = $1`, id)
	ta := adminToken(t, h)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/contracts/"+id+"/refund", tu, map[string]any{"kind": "full"}); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin refund %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/admin/coaching/contracts/"+id+"/refund", ta, map[string]any{"kind": "remaining"})
	if rec.Code != http.StatusOK {
		t.Fatalf("admin refund %d %s", rec.Code, rec.Body.String())
	}
	refunds = f.refundList()
	if len(refunds) != 2 || refunds[1].Get("payment_intent") != "pi_1" {
		t.Fatalf("admin refund %v", refunds)
	}
	if amount := refunds[1].Get("amount"); amount != "3999" && amount != "4000" {
		t.Fatalf("remaining refund %s", amount)
	}
	if contractStatus(t, id) != "refunded" {
		t.Fatalf("refunded status %s", contractStatus(t, id))
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/contracts/"+id+"/refund", ta, map[string]any{"kind": "full"}); rec.Code != http.StatusConflict {
		t.Fatalf("second refund %d", rec.Code)
	}
	// 返金して閉じたあとも、やりとりは読める。送れない。
	if rec := doJSON(t, h, http.MethodGet, "/v1/coaching/contracts/"+id+"/messages", tu, nil); rec.Code != http.StatusOK {
		t.Fatalf("read after refund %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/messages", tc, map[string]any{"body": "こんにちは"}); rec.Code != http.StatusConflict {
		t.Fatalf("send after refund %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/admin/coaching/contracts", ta, nil); !strings.Contains(rec.Body.String(), `"payment_status":"partially_refunded"`) {
		t.Fatalf("admin list %s", rec.Body.String())
	}
}

func TestCoachingPaymentRefundsWhenTheCoachIsSilent(t *testing.T) {
	h, f := setupStripe(t)
	tuA, tcA, a := adultAccepted(t, h)
	tuB, _, b := adultAccepted(t, h)
	checkout(t, h, tuA, a)
	checkout(t, h, tuB, b)
	paidSession(t, h, sessionIDFor(t, a), "pi_a", 8000)
	paidSession(t, h, sessionIDFor(t, b), "pi_b", 8000)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+a+"/messages", tcA, map[string]any{"body": "動画を送ってください"}); rec.Code != http.StatusCreated {
		t.Fatalf("coach message %d %s", rec.Code, rec.Body.String())
	}

	// 72時間たつ前は返金しない。
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"payments_refunded":0`) {
		t.Fatalf("tick early %d %s", rec.Code, rec.Body.String())
	}
	execSQLArgs(t, `UPDATE coaching_contracts SET starts_at = now() - interval '73 hours' WHERE id = ANY($1::uuid[])`, []string{a, b})
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"payments_refunded":1`) {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
	refunds := f.refundList()
	if len(refunds) != 1 || refunds[0].Get("payment_intent") != "pi_b" || refunds[0].Get("amount") != "8000" {
		t.Fatalf("silent refund %v", refunds)
	}
	if contractStatus(t, a) != "active" || contractStatus(t, b) != "refunded" {
		t.Fatalf("statuses %s %s", contractStatus(t, a), contractStatus(t, b))
	}
	var reason string
	queryRowArgs(t, `SELECT refund_reason FROM coaching_payments WHERE contract_id = $1`, []any{b}, &reason)
	if reason != "no_coach_reply" {
		t.Fatalf("reason %s", reason)
	}
}

func TestCoachingPaymentByGuardian(t *testing.T) {
	h, f := setupStripe(t)
	tm, gt := guardianMinor(t, h)
	_, id := minorAccepted(t, h, tm)
	gpath := "/v1/guardian/" + gt + "/contracts/" + id

	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/checkout", tm, nil); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "guardian_pays") {
		t.Fatalf("minor checkout %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, gpath+"/checkout", "", nil); rec.Code != http.StatusConflict {
		t.Fatalf("checkout before consent %d", rec.Code)
	}
	rec := doJSON(t, h, http.MethodPost, gpath+"/consent", "", map[string]any{"agree_report": true, "agree_detection": true})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"guardian_consented":true`) {
		t.Fatalf("guardian consent %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		URL      string       `json:"checkout_url"`
		Contract contractView `json:"contract"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.URL == "" || out.Contract.Status != "accepted" {
		t.Fatalf("consent result %+v", out)
	}
	// 保護者のリンクは Stripe に渡さない。
	form := f.lastSession()
	if form.Get("success_url") != "https://web.test/guardian/paid" || strings.Contains(form.Encode(), gt) {
		t.Fatalf("guardian session %v", form)
	}
	// もう一度同意しても、開き直しても、同じ支払い画面。
	rec = doJSON(t, h, http.MethodPost, gpath+"/consent", "", map[string]any{"agree_report": true, "agree_detection": true})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), out.URL) {
		t.Fatalf("consent again %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, gpath+"/checkout", "", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), out.URL) || len(f.sessions) != 1 {
		t.Fatalf("guardian checkout %d %s", rec.Code, rec.Body.String())
	}
	var payer string
	queryRowArgs(t, `SELECT payer FROM coaching_payments WHERE contract_id = $1`, []any{id}, &payer)
	if payer != "guardian" {
		t.Fatalf("payer %s", payer)
	}
	paidSession(t, h, sessionIDFor(t, id), "pi_g", 8000)
	if contractStatus(t, id) != "active" {
		t.Fatalf("guardian paid %s", contractStatus(t, id))
	}
}

func TestCoachingPaymentAfterTheDeadlineIsRefunded(t *testing.T) {
	h, f := setupStripe(t)
	tu, _, id := adultAccepted(t, h)
	checkout(t, h, tu, id)
	execSQLArgs(t, `UPDATE coaching_contracts SET start_by = now() - interval '1 minute' WHERE id = $1`, id)
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatalf("tick %d", rec.Code)
	}
	if contractStatus(t, id) != "expired" {
		t.Fatalf("expired %s", contractStatus(t, id))
	}
	paidSession(t, h, sessionIDFor(t, id), "pi_late", 8000)
	refunds := f.refundList()
	if len(refunds) != 1 || refunds[0].Get("payment_intent") != "pi_late" || refunds[0].Get("amount") != "8000" {
		t.Fatalf("late refund %v", refunds)
	}
	if contractStatus(t, id) != "expired" {
		t.Fatal("late payment started the contract")
	}

	// 鍵がなければ Webhook は受け付けず、承諾したらそのまま始まる（デモ版）。
	demo := setup(t)
	if rec := stripeWebhook(t, demo, testWebhookSecret, "checkout.session.completed", map[string]any{"id": "cs_x"}); rec.Code != http.StatusNotFound {
		t.Fatalf("demo webhook %d", rec.Code)
	}
}
