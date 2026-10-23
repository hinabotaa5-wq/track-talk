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
	// APIVersion を入れると Stripe-Version ヘッダーで API のバージョンを固定する。空ならアカウントの既定。
	APIVersion string
	HTTP       *http.Client
	// RetryDelay は、一時的な失敗のあとの最初の待ち時間(2回目はその2倍)。空なら400ms。
	RetryDelay time.Duration
}

// 同じ Idempotency-Key で送る POST は、一時的な失敗(通信エラー・429・5xx)をこの回数まで試す。
const maxAttempts = 3

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
// idempotencyKey がある POST は、一時的な失敗(通信エラー・429・5xx)なら間を置いて送り直す(重複して課金されない)。
func (c Client) post(ctx context.Context, path string, form url.Values, idempotencyKey string, out any) error {
	if !c.Configured() {
		return ErrNotConfigured
	}
	attempts := 1
	if idempotencyKey != "" {
		attempts = maxAttempts
	}
	delay := c.RetryDelay
	if delay <= 0 {
		delay = 400 * time.Millisecond
	}
	var err error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay << (i - 1)):
			}
		}
		err = c.postOnce(ctx, path, form, idempotencyKey, out)
		if err == nil || !retryable(ctx, err) {
			return err
		}
	}
	return err
}

func (c Client) postOnce(ctx context.Context, path string, form url.Values, idempotencyKey string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base()+path, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.Secret, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	return c.do(req, out)
}

// retryable は、送り直して良い失敗か。呼んだ側が止めた(ctx)ときと、Stripe が断った 4xx は送り直さない。
func retryable(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return false
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Status == http.StatusTooManyRequests || apiErr.Status >= 500
	}
	return true
}

// do は、リクエストを送って JSON を out に読む。2xx 以外は APIError。
func (c Client) do(req *http.Request, out any) error {
	if c.APIVersion != "" {
		req.Header.Set("Stripe-Version", c.APIVersion)
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
	// 空なら、ダッシュボードで有効にした支払い方法が出る。
	PaymentMethodTypes []string
	// true なら仮押さえだけして、あとで CapturePaymentIntent で確定する（カードだけ）。
	CaptureManual bool
}

type CheckoutSession struct {
	ID                string            `json:"id"`
	URL               string            `json:"url"`
	ExpiresAt         int64             `json:"expires_at"`
	PaymentStatus     string            `json:"payment_status"`
	PaymentIntent     string            `json:"payment_intent"`
	ClientReferenceID string            `json:"client_reference_id"`
	AmountTotal       int               `json:"amount_total"`
	Currency          string            `json:"currency"`
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
	for i, t := range p.PaymentMethodTypes {
		form.Set("payment_method_types["+strconv.Itoa(i)+"]", t)
	}
	if p.CaptureManual {
		form.Set("payment_intent_data[capture_method]", "manual")
	}
	var s CheckoutSession
	err := c.post(ctx, "/v1/checkout/sessions", form, p.IdempotencyKey, &s)
	return s, err
}

type Refund struct {
	Metadata map[string]string `json:"metadata"`
	ID       string            `json:"id"`
	Amount   int               `json:"amount"`
	Status   string            `json:"status"`
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

type PaymentIntent struct {
	ID string `json:"id"`
	// requires_capture: 仮押さえ済み。succeeded: 確定済み。canceled: 取り消し済み。
	Status string `json:"status"`
	Amount int    `json:"amount"`
}

// GetPaymentIntent は、支払いの今の状態を読む。
func (c Client) GetPaymentIntent(ctx context.Context, id string) (PaymentIntent, error) {
	if !c.Configured() {
		return PaymentIntent{}, ErrNotConfigured
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+"/v1/payment_intents/"+url.PathEscape(id), nil)
	if err != nil {
		return PaymentIntent{}, err
	}
	req.SetBasicAuth(c.Secret, "")
	var pi PaymentIntent
	err = c.do(req, &pi)
	return pi, err
}

// CapturePaymentIntent は、仮押さえした支払いを確定する。
func (c Client) CapturePaymentIntent(ctx context.Context, id, idempotencyKey string) (PaymentIntent, error) {
	var pi PaymentIntent
	err := c.post(ctx, "/v1/payment_intents/"+url.PathEscape(id)+"/capture", url.Values{}, idempotencyKey, &pi)
	return pi, err
}

// CancelPaymentIntent は、仮押さえを取り消す。確定前なので手数料はかからない。
func (c Client) CancelPaymentIntent(ctx context.Context, id, idempotencyKey string) (PaymentIntent, error) {
	var pi PaymentIntent
	err := c.post(ctx, "/v1/payment_intents/"+url.PathEscape(id)+"/cancel", url.Values{}, idempotencyKey, &pi)
	return pi, err
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

// Account は、コーチの Stripe Connect のアカウント（Express）。
type Account struct {
	ID               string `json:"id"`
	PayoutsEnabled   bool   `json:"payouts_enabled"`
	DetailsSubmitted bool   `json:"details_submitted"`
}

// CreateAccount は、コーチの受け取り用のアカウントを作る。本人確認と口座の登録は Stripe の画面で行う。
func (c Client) CreateAccount(ctx context.Context, metadata map[string]string, idempotencyKey string) (Account, error) {
	form := url.Values{}
	form.Set("type", "express")
	form.Set("country", "JP")
	form.Set("capabilities[transfers][requested]", "true")
	for k, v := range metadata {
		form.Set("metadata["+k+"]", v)
	}
	var a Account
	err := c.post(ctx, "/v1/accounts", form, idempotencyKey, &a)
	return a, err
}

// GetAccount は、アカウントの今の状態（送金を受け取れるか）を読む。
func (c Client) GetAccount(ctx context.Context, id string) (Account, error) {
	if !c.Configured() {
		return Account{}, ErrNotConfigured
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+"/v1/accounts/"+url.PathEscape(id), nil)
	if err != nil {
		return Account{}, err
	}
	req.SetBasicAuth(c.Secret, "")
	var a Account
	err = c.do(req, &a)
	return a, err
}

// CreateAccountLink は、口座登録の画面の URL を作る。URL は数分で切れるので、開くたびに作る。
func (c Client) CreateAccountLink(ctx context.Context, account, refreshURL, returnURL string) (string, error) {
	form := url.Values{}
	form.Set("account", account)
	form.Set("refresh_url", refreshURL)
	form.Set("return_url", returnURL)
	form.Set("type", "account_onboarding")
	var out struct {
		URL string `json:"url"`
	}
	err := c.post(ctx, "/v1/account_links", form, "", &out)
	return out.URL, err
}

type Transfer struct {
	ID     string `json:"id"`
	Amount int    `json:"amount"`
}

// CreateTransfer は、TrackTalk の残高からコーチのアカウントへ送金する。
// group は transfer_group として付け、送り直す前に FindTransfer で探せるようにする。
func (c Client) CreateTransfer(ctx context.Context, account string, amountYen int, group string, metadata map[string]string, idempotencyKey string) (Transfer, error) {
	form := url.Values{}
	form.Set("amount", strconv.Itoa(amountYen))
	form.Set("currency", "jpy")
	form.Set("destination", account)
	if group != "" {
		form.Set("transfer_group", group)
	}
	for k, v := range metadata {
		form.Set("metadata["+k+"]", v)
	}
	var t Transfer
	err := c.post(ctx, "/v1/transfers", form, idempotencyKey, &t)
	return t, err
}

// FindTransfers は、transfer_group が group の送金を返す。Idempotency-Key は約24時間で切れるので、
// 送り直す前にこれで「Stripe 側では送れていた」送金を探す。
func (c Client) FindTransfers(ctx context.Context, group string) ([]Transfer, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	q := url.Values{}
	q.Set("transfer_group", group)
	q.Set("limit", "10")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+"/v1/transfers?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.Secret, "")
	var out struct {
		Data []Transfer `json:"data"`
	}
	err = c.do(req, &out)
	return out.Data, err
}
