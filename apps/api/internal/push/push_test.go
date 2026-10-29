package push

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

func testSubscription(t *testing.T, endpoint string) Subscription {
	t.Helper()
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 16)
	_, _ = rand.Read(auth)
	return Subscription{
		Endpoint: endpoint,
		P256dh:   base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()),
		Auth:     base64.RawURLEncoding.EncodeToString(auth),
	}
}

func testSender(t *testing.T) Sender {
	t.Helper()
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	return Sender{PublicKey: pub, PrivateKey: priv, Subject: "mailto:test@example.com"}
}

func TestSendReportsGoneSubscriptions(t *testing.T) {
	status := http.StatusCreated
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.WriteHeader(status)
	}))
	defer srv.Close()
	s := testSender(t)
	sub := testSubscription(t, srv.URL+"/push/abc")
	msg := Message{Title: "添削の依頼が届きました", Body: "30分以内に引き受けてください", URL: "/coach"}
	ctx := context.Background()

	if err := s.Send(ctx, sub, msg, time.Minute); err != nil {
		t.Fatalf("send: %v", err)
	}
	if !strings.HasPrefix(auth, "vapid t=") {
		t.Fatalf("authorization %q", auth)
	}
	status = http.StatusGone
	if err := s.Send(ctx, sub, msg, time.Minute); err != ErrGone {
		t.Fatalf("gone: %v", err)
	}
	status = http.StatusNotFound
	if err := s.Send(ctx, sub, msg, time.Minute); err != ErrGone {
		t.Fatalf("not found: %v", err)
	}
	status = http.StatusInternalServerError
	if err := s.Send(ctx, sub, msg, time.Minute); !errors.Is(err, ErrFailed) || !strings.Contains(err.Error(), "status 500") {
		t.Fatalf("server error: %v", err)
	}
	if err := (Sender{}).Send(ctx, sub, msg, time.Minute); err != ErrNotConfigured {
		t.Fatalf("unconfigured: %v", err)
	}
}
