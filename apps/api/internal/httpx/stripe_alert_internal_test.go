package httpx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/mail"
	"github.com/hinata/track-talk/apps/api/internal/stripe"
)

func TestParseRefundObject(t *testing.T) {
	// refund.created は Refund オブジェクト。
	r, err := parseRefundObject(json.RawMessage(`{"id":"re_1","amount":500,"payment_intent":"pi_1","charge":"ch_1"}`))
	if err != nil || r.RefundID != "re_1" || r.PaymentIntent != "pi_1" || r.Amount != 500 {
		t.Fatalf("refund: %+v err=%v", r, err)
	}
	// charge.refunded は Charge オブジェクト。累計の返金額と、返金の一覧が来る。
	r, err = parseRefundObject(json.RawMessage(`{"id":"ch_1","amount_refunded":800,"payment_intent":"pi_2","refunds":{"data":[{"id":"re_2","amount":800}]}}`))
	if err != nil || r.RefundID != "re_2" || r.PaymentIntent != "pi_2" || r.Amount != 800 {
		t.Fatalf("charge: %+v err=%v", r, err)
	}
	if _, err := parseRefundObject(json.RawMessage(`{`)); err == nil {
		t.Fatal("expected error for broken JSON")
	}
}

func TestParseTransferObject(t *testing.T) {
	tr, err := parseTransferObject(json.RawMessage(`{"id":"tr_1","amount":3000,"destination":"acct_1","metadata":{"payout_id":"p1"}}`))
	if err != nil || tr.ID != "tr_1" || tr.Amount != 3000 || tr.Destination != "acct_1" || tr.Metadata["payout_id"] != "p1" {
		t.Fatalf("transfer: %+v err=%v", tr, err)
	}
}

func TestRefundKnownTable(t *testing.T) {
	cases := []struct {
		name            string
		byID, byPayment bool
		want            bool
	}{
		{"記録なし", false, false, false},
		{"返金IDの記録あり", true, false, true},
		{"支払いに返金記録あり", false, true, true},
		{"両方あり", true, true, true},
	}
	for _, tc := range cases {
		if got := refundKnown(tc.byID, tc.byPayment); got != tc.want {
			t.Fatalf("%s: refundKnown(%v,%v)=%v want %v", tc.name, tc.byID, tc.byPayment, got, tc.want)
		}
	}
}

func TestTransferKnownTable(t *testing.T) {
	n := transferNotice{ID: "tr_1", Amount: 3000, Destination: "acct_1", Metadata: map[string]string{"payout_id": "p1"}}
	record := &payoutRecord{TransferID: "", AmountYen: 3000, StripeAccountID: "acct_1"}
	cases := []struct {
		name                     string
		byTransferID, byPayoutID *payoutRecord
		want                     bool
	}{
		{"記録なし", nil, nil, false},
		{"送金IDの記録あり", record, nil, true},
		{"payout_id・金額・宛先が一致", nil, record, true},
		{"金額違い", nil, &payoutRecord{AmountYen: 2000, StripeAccountID: "acct_1"}, false},
		{"宛先違い", nil, &payoutRecord{AmountYen: 3000, StripeAccountID: "acct_other"}, false},
	}
	for _, tc := range cases {
		if got := transferKnown(tc.byTransferID, tc.byPayoutID, n); got != tc.want {
			t.Fatalf("%s: transferKnown=%v want %v", tc.name, got, tc.want)
		}
	}
}

// payout の宛先チェック: 一致／不一致／未設定の3ケース。
func TestCheckPayout(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"id":"po_1","destination":{"object":"bank_account","last4":"4242"}}`))
	}))
	defer srv.Close()
	newServer := func(expected string) *Server {
		return &Server{
			cfg:    config.Config{OpsAlertEmail: "ops@example.com", OpsPayoutExpectedLast4: expected},
			mail:   mail.Client{DevLog: true},
			stripe: stripe.Client{Secret: "sk_test_x", APIBase: srv.URL},
		}
	}
	ctx := context.Background()
	// 未設定: ログのみ。Stripe へも聞かない。
	calls = 0
	if newServer("").checkPayout(ctx, "po_1", 3000, time.Now()) {
		t.Fatal("unset: alerted")
	}
	if calls != 0 {
		t.Fatalf("unset: stripe called %d times", calls)
	}
	// 一致: 知らせない。
	if newServer("4242").checkPayout(ctx, "po_1", 3000, time.Now()) {
		t.Fatal("match: alerted")
	}
	// 不一致: 知らせる。
	if !newServer("9999").checkPayout(ctx, "po_1", 3000, time.Now()) {
		t.Fatal("mismatch: not alerted")
	}
	if !payoutLast4Mismatch("9999", "4242") || payoutLast4Mismatch("", "4242") || payoutLast4Mismatch("9999", "") {
		t.Fatal("payoutLast4Mismatch table")
	}
}

// OPS_ALERT_EMAIL 未設定・メール未設定では、知らせは送らない（落とさず false）。
func TestAlertOpsNotConfigured(t *testing.T) {
	ctx := context.Background()
	// 宛先が無い。
	s := &Server{cfg: config.Config{}, mail: mail.Client{DevLog: true}}
	if s.alertOps(ctx, "件名", "本文", "key-1") {
		t.Fatal("no recipient: alerted")
	}
	// 宛先はあるがメール設定が無い。
	s = &Server{cfg: config.Config{OpsAlertEmail: "ops@example.com"}}
	if s.alertOps(ctx, "件名", "本文", "key-2") {
		t.Fatal("mail not configured: alerted")
	}
}
