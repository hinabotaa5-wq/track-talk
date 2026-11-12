package push

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
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

func TestMessageUnreadCountJSON(t *testing.T) {
	// 未読数は付けたときだけ送り、付けない経路では項目自体を出さない。
	// 0 も「未読なし」の意味があるので省かない。
	out, err := json.Marshal(Message{Title: "T", Body: "B", URL: "/", UnreadCount: ptrInt64(3)})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"unread_count":3`) {
		t.Fatalf("payload %s", out)
	}
	out, err = json.Marshal(Message{Title: "T", Body: "B", URL: "/", UnreadCount: ptrInt64(0)})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"unread_count":0`) {
		t.Fatalf("payload %s", out)
	}
	out, err = json.Marshal(Message{Title: "T", Body: "B", URL: "/"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "unread_count") {
		t.Fatalf("payload %s", out)
	}
	var back Message
	if err := json.Unmarshal([]byte(`{"title":"T","body":"B","url":"/","subscription_id":"s","unread_count":3}`), &back); err != nil {
		t.Fatal(err)
	}
	if back.UnreadCount == nil || *back.UnreadCount != 3 {
		t.Fatalf("decoded %+v", back)
	}
}

func ptrInt64(v int64) *int64 { return &v }

func TestSendReportsGoneSubscriptions(t *testing.T) {
	status := http.StatusCreated
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.WriteHeader(status)
		if status >= 400 {
			_, _ = w.Write([]byte("provider-response-with-fixture-endpoint"))
		}
	}))
	defer srv.Close()
	s := testSender(t)
	sub := testSubscription(t, srv.URL+"/push/abc")
	msg := Message{Title: "添削の依頼が届きました", Body: "90分以内に引き受けてください", URL: "/coach"}
	ctx := context.Background()

	if err := s.Send(ctx, sub, msg, time.Minute); err != nil {
		t.Fatalf("send: %v", err)
	}
	if !strings.HasPrefix(auth, "vapid t=") {
		t.Fatalf("authorization %q", auth)
	}
	// 設定の mailto: が二重にならない(Apple は mailto:mailto: を BadJwtToken で拒否する)。
	token := strings.TrimPrefix(strings.SplitN(auth, ",", 2)[0], "vapid t=")
	claims, err := base64.RawURLEncoding.DecodeString(strings.Split(token, ".")[1])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(claims), `"sub":"mailto:test@example.com"`) {
		t.Fatalf("claims %s", claims)
	}
	status = http.StatusGone
	if err := s.Send(ctx, sub, msg, time.Minute); !errors.Is(err, ErrGone) {
		t.Fatalf("gone: %v", err)
	}
	status = http.StatusNotFound
	if err := s.Send(ctx, sub, msg, time.Minute); !errors.Is(err, ErrGone) {
		t.Fatalf("not found: %v", err)
	}
	// 認証の拒否と宛先の失効は区別する。
	status = http.StatusForbidden
	if err := s.Send(ctx, sub, msg, time.Minute); !errors.Is(err, ErrRejected) || err.Error() != "push authorization rejected: status 403" {
		t.Fatalf("forbidden: %v", err)
	}
	status = http.StatusInternalServerError
	if err := s.Send(ctx, sub, msg, time.Minute); !errors.Is(err, ErrFailed) || err.Error() != "push failed: status 500" {
		t.Fatalf("server error: %v", err)
	}
	if err := (Sender{}).Send(ctx, sub, msg, time.Minute); err != ErrNotConfigured {
		t.Fatalf("unconfigured: %v", err)
	}
}
