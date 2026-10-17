// Package stripe は、パーソナル指導の支払いに使う Stripe の API のうち、必要なものだけを呼ぶ。
// Checkout（支払い画面）を作る、返金する、Webhook の署名を確かめる、の3つ。
package stripe

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultAPIBase = "https://api.stripe.com"

// Webhook の署名の時刻がこれより古いものは受け付けない（Stripe の既定と同じ5分）。
const webhookTolerance = 5 * time.Minute

var (
	ErrNotConfigured = errors.New("stripe not configured")
	ErrBadSignature  = errors.New("stripe webhook signature invalid")
)

type Client struct {
	Secret  string
	APIBase string
	HTTP    *http.Client
}

// Configured は、秘密鍵が設定されているか。設定されていなければ支払いなしのデモ版として動く。
func (c Client) Configured() bool { return c.Secret != "" }

func (c Client) base() string {
	if c.APIBase != "" {
		return strings.TrimRight(c.APIBase, "/")
	}
	return defaultAPIBase
}

func (c Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 20 * time.Second}
}

// APIError は、Stripe が 2xx 以外を返したとき。
type APIError struct {
	Status  int
	Type    string
	Code    string
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("stripe: %d %s %s: %s", e.Status, e.Type, e.Code, e.Message)
}

// post は、フォーム形式で POST する。同じ idempotencyKey で送り直すと、Stripe は前と同じ結果を返す。
func (c Client) post(ctx context.Context, path string, form url.Values, idempotencyKey string, out any) error {
	if !c.Configured() {
		return ErrNotConfigured
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base()+path, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.Secret, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	res, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	if res.StatusCode/100 != 2 {
		var e struct {
			Error struct {
				Type    string `json:"type"`
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(body, &e)
		return &APIError{Status: res.StatusCode, Type: e.Error.Type, Code: e.Error.Code, Message: e.Error.Message}
	}
	return json.Unmarshal(body, out)
}

type CheckoutParams struct {
	// 商品名として支払い画面に出る。
	Name       string
	AmountYen  int
	SuccessURL string
	CancelURL  string
	// 支払いの行の ID。Webhook で支払いを探すのに使う。
	Reference string
	Metadata  map[string]string
	ExpiresAt time.Time
	// 同じ値で送り直しても、支払い画面は1つしか作られない。
	IdempotencyKey string
}

type CheckoutSession struct {
	ID                string            `json:"id"`
	URL               string            `json:"url"`
	ExpiresAt         int64             `json:"expires_at"`
	PaymentStatus     string            `json:"payment_status"`
	PaymentIntent     string            `json:"payment_intent"`
	ClientReferenceID string            `json:"client_reference_id"`
	AmountTotal       int               `json:"amount_total"`
	Metadata          map[string]string `json:"metadata"`
}

// CreateCheckoutSession は、1回払いの支払い画面を作る。支払い方法（カード・Apple Pay・Google Pay・PayPay）は
// Stripe のダッシュボードで有効にしたものが出る。
func (c Client) CreateCheckoutSession(ctx context.Context, p CheckoutParams) (CheckoutSession, error) {
	form := url.Values{}
	form.Set("mode", "payment")
	form.Set("locale", "ja")
	form.Set("line_items[0][quantity]", "1")
	form.Set("line_items[0][price_data][currency]", "jpy")
	form.Set("line_items[0][price_data][unit_amount]", strconv.Itoa(p.AmountYen))
	form.Set("line_items[0][price_data][product_data][name]", p.Name)
	form.Set("success_url", p.SuccessURL)
	form.Set("cancel_url", p.CancelURL)
	form.Set("client_reference_id", p.Reference)
	form.Set("expires_at", strconv.FormatInt(p.ExpiresAt.Unix(), 10))
	for k, v := range p.Metadata {
		form.Set("metadata["+k+"]", v)
		form.Set("payment_intent_data[metadata]["+k+"]", v)
	}
	var s CheckoutSession
	err := c.post(ctx, "/v1/checkout/sessions", form, p.IdempotencyKey, &s)
	return s, err
}

type Refund struct {
	ID     string `json:"id"`
	Amount int    `json:"amount"`
	Status string `json:"status"`
}

// CreateRefund は、支払いの一部か全部を返金する。
func (c Client) CreateRefund(ctx context.Context, paymentIntent string, amountYen int, idempotencyKey string) (Refund, error) {
	form := url.Values{}
	form.Set("payment_intent", paymentIntent)
	form.Set("amount", strconv.Itoa(amountYen))
	var r Refund
	err := c.post(ctx, "/v1/refunds", form, idempotencyKey, &r)
	return r, err
}

type Event struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Data struct {
		Object json.RawMessage `json:"object"`
	} `json:"data"`
}

// VerifyWebhook は、Stripe-Signature ヘッダーを確かめて、イベントを読む。
// ヘッダーは "t=時刻,v1=署名,..."。署名は "時刻.本文" の HMAC-SHA256。
func VerifyWebhook(payload []byte, header, secret string, now time.Time) (Event, error) {
	if secret == "" {
		return Event{}, ErrNotConfigured
	}
	var ts int64
	var sigs [][]byte
	for _, part := range strings.Split(header, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch k {
		case "t":
			ts, _ = strconv.ParseInt(v, 10, 64)
		case "v1":
			if b, err := hex.DecodeString(v); err == nil {
				sigs = append(sigs, b)
			}
		}
	}
	if ts == 0 || len(sigs) == 0 {
		return Event{}, ErrBadSignature
	}
	if d := now.Sub(time.Unix(ts, 0)); d > webhookTolerance || d < -webhookTolerance {
		return Event{}, ErrBadSignature
	}
	want := Sign(payload, secret, ts)
	ok := false
	for _, s := range sigs {
		if hmac.Equal(s, want) {
			ok = true
		}
	}
	if !ok {
		return Event{}, ErrBadSignature
	}
	var ev Event
	if err := json.Unmarshal(payload, &ev); err != nil {
		return Event{}, err
	}
	return ev, nil
}

// Sign は、Webhook の署名を作る。テストで Stripe の代わりに署名するのにも使う。
func Sign(payload []byte, secret string, ts int64) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(ts, 10)))
	mac.Write([]byte("."))
	mac.Write(payload)
	return mac.Sum(nil)
}
