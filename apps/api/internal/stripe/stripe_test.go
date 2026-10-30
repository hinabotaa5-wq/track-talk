package stripe

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestVerifyWebhook(t *testing.T) {
	payload := []byte(`{"id":"evt_1","type":"checkout.session.completed","data":{"object":{"id":"cs_1"}}}`)
	now := time.Unix(1_800_000_000, 0)
	sig := hex.EncodeToString(Sign(payload, "whsec_x", now.Unix()))
	header := fmt.Sprintf("t=%d,v1=deadbeef,v1=%s", now.Unix(), sig)

	ev, err := VerifyWebhook(payload, header, "whsec_x", now.Add(time.Minute))
	if err != nil || ev.Type != "checkout.session.completed" || ev.ID != "evt_1" {
		t.Fatalf("valid: %+v %v", ev, err)
	}
	if _, err := VerifyWebhook(payload, header, "whsec_other", now); err != ErrBadSignature {
		t.Fatalf("wrong secret: %v", err)
	}
	if _, err := VerifyWebhook(append(payload, ' '), header, "whsec_x", now); err != ErrBadSignature {
		t.Fatalf("tampered: %v", err)
	}
	if _, err := VerifyWebhook(payload, header, "whsec_x", now.Add(10*time.Minute)); err != ErrBadSignature {
		t.Fatalf("too old: %v", err)
	}
	if _, err := VerifyWebhook(payload, "", "whsec_x", now); err != ErrBadSignature {
		t.Fatalf("no header: %v", err)
	}
}

func TestPostRetriesTransientFailuresOnlyWithIdempotencyKey(t *testing.T) {
	var calls int
	var versions, keys []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		versions = append(versions, r.Header.Get("Stripe-Version"))
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		if calls < 3 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":{"type":"api_error","message":"boom"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"re_1","amount":500,"status":"succeeded"}`))
	}))
	defer srv.Close()
	c := Client{Secret: "sk_test_x", APIBase: srv.URL, APIVersion: "2099-01-01", RetryDelay: time.Millisecond}

	r, err := c.CreateRefund(context.Background(), "pi_1", 500, "key-1")
	if err != nil || r.ID != "re_1" || calls != 3 {
		t.Fatalf("retry: %+v %v calls=%d", r, err, calls)
	}
	for i := range keys {
		if keys[i] != "key-1" || versions[i] != "2099-01-01" {
			t.Fatalf("headers %v %v", keys, versions)
		}
	}

	// キーが無い POST は、二重に実行されるおそれがあるので送り直さない。
	calls = 0
	if _, err := c.CreateRefund(context.Background(), "pi_1", 500, ""); err == nil || calls != 1 {
		t.Fatalf("no key: %v calls=%d", err, calls)
	}
}

func TestPostDoesNotRetryClientErrors(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"type":"invalid_request_error","code":"amount_too_small","message":"x"}}`))
	}))
	defer srv.Close()
	c := Client{Secret: "sk_test_x", APIBase: srv.URL, RetryDelay: time.Millisecond}
	_, err := c.CreateRefund(context.Background(), "pi_1", 1, "key-2")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "amount_too_small" || calls != 1 {
		t.Fatalf("4xx: %v calls=%d", err, calls)
	}
}

func TestListRefundsAndTransfers(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path+"?"+r.URL.RawQuery)
		if r.URL.Path == "/v1/refunds" {
			_, _ = w.Write([]byte(`{"data":[{"id":"re_1","amount":500,"payment_intent":"pi_1"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"tr_1","amount":3000,"destination":"acct_1","metadata":{"payout_id":"p1"}}]}`))
	}))
	defer srv.Close()
	c := Client{Secret: "sk_test_x", APIBase: srv.URL, RetryDelay: time.Millisecond}
	since := time.Unix(1700000000, 0)
	refunds, err := c.ListRefunds(context.Background(), since)
	if err != nil || len(refunds) != 1 || refunds[0].PaymentIntent != "pi_1" {
		t.Fatalf("refunds: %+v %v", refunds, err)
	}
	transfers, err := c.ListTransfers(context.Background(), since)
	if err != nil || len(transfers) != 1 || transfers[0].Destination != "acct_1" || transfers[0].Metadata["payout_id"] != "p1" {
		t.Fatalf("transfers: %+v %v", transfers, err)
	}
	for _, p := range paths {
		if !strings.Contains(p, "created%5Bgte%5D=1700000000") {
			t.Fatalf("missing created filter: %s", p)
		}
	}
}

func TestGetPayoutDestinationLast4(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "expand") {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"id":"po_1","destination":{"object":"bank_account","last4":"4242"}}`))
	}))
	defer srv.Close()
	c := Client{Secret: "sk_test_x", APIBase: srv.URL, RetryDelay: time.Millisecond}
	last4, err := c.GetPayoutDestinationLast4(context.Background(), "po_1")
	if err != nil || last4 != "4242" {
		t.Fatalf("last4=%q err=%v", last4, err)
	}
	// 展開されず ID の文字列だけのときは、末尾4桁は不明として空を返す。
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"po_1","destination":"ba_1"}`))
	}))
	defer srv2.Close()
	c2 := Client{Secret: "sk_test_x", APIBase: srv2.URL, RetryDelay: time.Millisecond}
	if last4, err := c2.GetPayoutDestinationLast4(context.Background(), "po_1"); err != nil || last4 != "" {
		t.Fatalf("unexpanded: last4=%q err=%v", last4, err)
	}
}
