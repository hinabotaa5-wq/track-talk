// Package mail は、API からメールを送る。いまは保護者の同意のリンクだけで、送信には Resend の API を使う。
package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const defaultAPIBase = "https://api.resend.com"

var ErrNotConfigured = errors.New("mail not configured")

type Message struct {
	To      string
	Subject string
	Text    string
}

type Client struct {
	// APIKey は Resend の API キー。
	APIKey string
	// From は送り主（例: "TrackTalk <no-reply@example.com>"）。Resend で認証したドメインのもの。
	From string
	// APIBase はテストで Resend の代わりのサーバーに向けるためのもの。
	APIBase string
	// DevLog が true なら送らずに本文をログに出す。手元で試すためのもので、本番では使わない。
	DevLog bool
	HTTP   *http.Client
}

// Configured は、メールを送れる（または手元でログに出せる）か。
func (c Client) Configured() bool { return c.DevLog || (c.APIKey != "" && c.From != "") }

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
	return &http.Client{Timeout: 15 * time.Second}
}

// Send は1通送る。idempotencyKey が同じなら、Resend は24時間のあいだ二重に送らない。
func (c Client) Send(ctx context.Context, m Message, idempotencyKey string) error {
	if !c.Configured() {
		return ErrNotConfigured
	}
	if c.DevLog {
		slog.Info("mail (dev log, not sent)", "to", m.To, "subject", m.Subject, "text", m.Text)
		return nil
	}
	body, err := json.Marshal(map[string]any{
		"from":    c.From,
		"to":      []string{m.To},
		"subject": m.Subject,
		"text":    m.Text,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base()+"/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	res, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		detail, _ := io.ReadAll(io.LimitReader(res.Body, 1000))
		return fmt.Errorf("resend: %d %s", res.StatusCode, strings.TrimSpace(string(detail)))
	}
	_, _ = io.Copy(io.Discard, res.Body)
	return nil
}
