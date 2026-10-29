package httpx_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/stripe"
	"github.com/jackc/pgx/v5/pgxpool"
)

const connectWebhookPath = "/webhooks/stripe/connect"

func TestStripeConnectWebhookWhileCoachingIsOff(t *testing.T) {
	h, f := setupStripeConfig(t, func(c *config.Config) {
		c.CoachingEnabled = false
		c.CoachingNewAcceptance = false
		c.CoachingMessaging = false
	})
	id := uuid.New()
	tok := token(t, id)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", tok, coachApplicationBody()); rec.Code != http.StatusOK {
		t.Fatalf("apply %d", rec.Code)
	}
	account := "acct_" + id.String()
	execSQLArgs(t, `UPDATE coach_profiles SET stripe_account_id = $2 WHERE profile_id = $1`, id, account)
	enabled := func() bool {
		var ok bool
		queryRowArgs(t, `SELECT payouts_enabled FROM coach_profiles WHERE profile_id = $1`, []any{id}, &ok)
		return ok
	}
	// 同一イベントを再送する。payload の状態でなく、Stripe の現在の状態が保存される。
	payload := fmt.Sprintf(`{"id":"evt_connect_duplicate","type":"account.updated","data":{"object":{"id":%q,"payouts_enabled":false}}}`, account)
	send := func() int {
		ts := time.Now().Unix()
		req := httptest.NewRequest(http.MethodPost, connectWebhookPath, strings.NewReader(payload))
		req.Header.Set("Stripe-Signature", fmt.Sprintf("t=%d,v1=%s", ts, hex.EncodeToString(stripe.Sign([]byte(payload), testConnectWebhookSecret, ts))))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	f.setEnabled(account, true)
	for range 2 {
		if code := send(); code != http.StatusOK || !enabled() {
			t.Fatalf("duplicate/stale enable %d %v", code, enabled())
		}
	}
	f.setEnabled(account, false)
	if code := send(); code != http.StatusOK || enabled() {
		t.Fatalf("disable %d %v", code, enabled())
	}
	if rec := stripeWebhookAt(t, h, connectWebhookPath, testConnectWebhookSecret, "account.updated", map[string]any{"id": "acct_unknown"}); rec.Code != http.StatusOK {
		t.Fatalf("unknown account %d", rec.Code)
	}
	for _, key := range []string{testWebhookSecret, "whsec_wrong"} {
		if rec := stripeWebhookAt(t, h, connectWebhookPath, key, "account.updated", map[string]any{"id": account}); rec.Code != http.StatusBadRequest {
			t.Fatalf("wrong key %d", rec.Code)
		}
	}
	f.mu.Lock()
	f.failAccountReads = true
	f.mu.Unlock()
	if code := send(); code != http.StatusInternalServerError || enabled() {
		t.Fatalf("retryable failure %d %v", code, enabled())
	}
	f.mu.Lock()
	f.failAccountReads = false
	f.mu.Unlock()
	f.setEnabled(account, true)
	if code := send(); code != http.StatusOK || !enabled() {
		t.Fatalf("retry %d %v", code, enabled())
	}
	// 先行する更新が行ロックを持つ間は、Stripe の読み直しも待たせる。
	// 待つ前に古い状態を読んでしまう実装への退行を検出する。
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT profile_id FROM coach_profiles WHERE stripe_account_id = $1 FOR UPDATE`, account); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	readsBefore := f.accountReads
	f.mu.Unlock()
	done := make(chan int, 1)
	go func() { done <- send() }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var waiting bool
		err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND query LIKE 'SELECT profile_id FROM coach_profiles WHERE stripe_account_id%')`).Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("webhook did not wait for the account row lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	f.mu.Lock()
	readsWhileLocked := f.accountReads
	f.mu.Unlock()
	if readsWhileLocked != readsBefore {
		t.Fatal("Stripe account was read before acquiring the row lock")
	}
	f.setEnabled(account, false)
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-done:
		if code != http.StatusOK || enabled() {
			t.Fatalf("serialized update %d %v", code, enabled())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("webhook did not complete after releasing the row lock")
	}
	for _, typ := range []string{"checkout.session.completed", "checkout.session.async_payment_succeeded", "charge.dispute.created"} {
		if rec := stripeWebhookAt(t, h, connectWebhookPath, testConnectWebhookSecret, typ, map[string]any{}); rec.Code != http.StatusBadRequest {
			t.Fatalf("connect accepted payment event %s: %d", typ, rec.Code)
		}
		if rec := stripeWebhook(t, h, testWebhookSecret, typ, map[string]any{}); rec.Code != http.StatusNotFound {
			t.Fatalf("payment route while off %s: %d", typ, rec.Code)
		}
	}
	for _, path := range []string{"/v1/coach/stripe/onboarding", "/internal/coaching/tick"} {
		if rec := doJSON(t, h, http.MethodPost, path, tok, nil); rec.Code != http.StatusNotFound {
			t.Fatalf("coaching route opened %s: %d", path, rec.Code)
		}
	}
	if len(f.sessions) != 0 || len(f.transfers) != 0 || len(f.refunds) != 0 {
		t.Fatal("connect webhook performed payment operations")
	}
}

func TestStripeConnectWebhookRequiresBothKeys(t *testing.T) {
	for _, missing := range []string{"stripe", "connect"} {
		t.Run(missing, func(t *testing.T) {
			h, _ := setupStripeConfig(t, func(c *config.Config) {
				c.CoachingEnabled = false
				if missing == "stripe" {
					c.StripeSecretKey = ""
				} else {
					c.StripeConnectWebhookSecret = ""
				}
			})
			if rec := stripeWebhookAt(t, h, connectWebhookPath, testConnectWebhookSecret, "account.updated", map[string]any{"id": "acct_unknown"}); rec.Code != http.StatusNotFound {
				t.Fatalf("missing key %d", rec.Code)
			}
		})
	}
}
