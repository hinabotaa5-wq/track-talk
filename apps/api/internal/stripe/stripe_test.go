package stripe

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
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
