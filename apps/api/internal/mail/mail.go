// Package mail は、API からメールを送る。いまは保護者の同意のリンクだけ。
// 送信は SMTP(Gmail など。独自ドメインが要らない)か Resend の API。SMTP が設定されていれば SMTP を使う。
package mail

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
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
	// SMTPHost・SMTPPort・SMTPUser・SMTPPassword を入れると、Resend ではなく SMTP(ポート465は最初から TLS、ほかは STARTTLS)で送る。
	// Gmail なら smtp.gmail.com:465 と、アプリパスワードを使う。
	SMTPHost     string
	SMTPPort     string
	SMTPUser     string
	SMTPPassword string
	// From は送り主（例: "TrackTalk <no-reply@example.com>"）。Resend で認証したドメインのもの。
	From string
	// APIBase はテストで Resend の代わりのサーバーに向けるためのもの。
	APIBase string
	// DevLog が true なら送らずに本文をログに出す。手元で試すためのもので、本番では使わない。
	DevLog bool
	HTTP   *http.Client
}

// Configured は、メールを送れる（または手元でログに出せる）か。
func (c Client) useSMTP() bool { return c.SMTPHost != "" }

func (c Client) Configured() bool {
	if c.useSMTP() {
		return c.SMTPUser != "" && c.SMTPPassword != "" && c.From != ""
	}
	return c.DevLog || (c.APIKey != "" && c.From != "")
}

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
	if c.useSMTP() {
		return c.sendSMTP(ctx, m)
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

// smtpMessage はメール本体(ヘッダーと本文)を組み立てる。件名と本文は UTF-8 で、件名は RFC 2047 で包む。
func smtpMessage(from, to, subject, text string) []byte {
	header := func(s string) string { return strings.NewReplacer("\r", " ", "\n", " ").Replace(s) }
	var b bytes.Buffer
	b.WriteString("From: " + header(from) + "\r\n")
	b.WriteString("To: " + header(to) + "\r\n")
	b.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", header(subject)) + "\r\n")
	b.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	enc := base64.StdEncoding.EncodeToString([]byte(text))
	for len(enc) > 76 {
		b.WriteString(enc[:76] + "\r\n")
		enc = enc[76:]
	}
	b.WriteString(enc + "\r\n")
	return b.Bytes()
}

func (c Client) sendSMTP(ctx context.Context, m Message) error {
	from, err := mail.ParseAddress(c.From)
	if err != nil {
		return fmt.Errorf("smtp: from: %w", err)
	}
	to, err := mail.ParseAddress(m.To)
	if err != nil {
		return fmt.Errorf("smtp: to: %w", err)
	}
	port := c.SMTPPort
	if port == "" {
		port = "465"
	}
	addr := net.JoinHostPort(c.SMTPHost, port)
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("smtp: %w", err)
	}
	tlsConfig := &tls.Config{ServerName: c.SMTPHost, MinVersion: tls.VersionTLS12}
	if port == "465" {
		conn = tls.Client(conn, tlsConfig)
	}
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	client, err := smtp.NewClient(conn, c.SMTPHost)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp: %w", err)
	}
	defer client.Close()
	if port != "465" {
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("smtp: starttls: %w", err)
		}
	}
	if err := client.Auth(smtp.PlainAuth("", c.SMTPUser, c.SMTPPassword, c.SMTPHost)); err != nil {
		return fmt.Errorf("smtp: auth: %w", err)
	}
	if err := client.Mail(from.Address); err != nil {
		return fmt.Errorf("smtp: %w", err)
	}
	if err := client.Rcpt(to.Address); err != nil {
		return fmt.Errorf("smtp: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp: %w", err)
	}
	if _, err := w.Write(smtpMessage(c.From, m.To, m.Subject, m.Text)); err != nil {
		return fmt.Errorf("smtp: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp: %w", err)
	}
	return client.Quit()
}
