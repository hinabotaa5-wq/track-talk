package mail

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSend(t *testing.T) {
	var got struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		Text    string   `json:"text"`
	}
	var auth, idem string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/emails" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		auth, idem = r.Header.Get("Authorization"), r.Header.Get("Idempotency-Key")
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"id":"em_1"}`))
	}))
	defer srv.Close()

	c := Client{APIKey: "re_test", From: "TrackTalk <no-reply@example.com>", APIBase: srv.URL}
	if err := c.Send(context.Background(), Message{To: "parent@example.com", Subject: "件名", Text: "本文"}, "key-1"); err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer re_test" || idem != "key-1" || got.From != c.From || len(got.To) != 1 || got.To[0] != "parent@example.com" || got.Subject != "件名" || got.Text != "本文" {
		t.Fatalf("request %q %q %+v", auth, idem, got)
	}

	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"invalid"}`, http.StatusUnprocessableEntity)
	}))
	defer failing.Close()
	c.APIBase = failing.URL
	if err := c.Send(context.Background(), Message{To: "parent@example.com"}, ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestNotConfigured(t *testing.T) {
	if err := (Client{APIKey: "re_test"}).Send(context.Background(), Message{}, ""); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("no from: %v", err)
	}
	if err := (Client{DevLog: true}).Send(context.Background(), Message{To: "parent@example.com"}, ""); err != nil {
		t.Fatalf("dev log: %v", err)
	}
}

func TestSMTPConfiguredAndMessage(t *testing.T) {
	c := Client{SMTPHost: "smtp.gmail.com", SMTPUser: "u@gmail.com", From: "TrackTalk <u@gmail.com>"}
	if c.Configured() {
		t.Fatal("no password should not be configured")
	}
	c.SMTPPassword = "app-password"
	if !c.Configured() {
		t.Fatal("smtp should be configured")
	}
	msg := string(smtpMessage(c.From, "parent@example.com", "同意のお願い\r\nBcc: x@example.com", "本文です"))
	if !strings.Contains(msg, "Subject: =?utf-8?q?") || strings.Contains(msg, "\r\nBcc:") || !strings.Contains(msg, "To: parent@example.com\r\n") {
		t.Fatalf("message %q", msg)
	}
}
