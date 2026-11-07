package httpx_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

func TestPushSubscriptionDelivery(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	const providerDetail = "fixture-provider-detail"

	var attempts atomic.Int32
	var reject atomic.Int32
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	defer close(release)
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		if status := reject.Swap(0); status != 0 {
			started <- struct{}{}
			<-release
			w.WriteHeader(int(status))
			_, _ = w.Write([]byte(providerDetail + " https://" + r.Host + r.URL.Path))
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(provider.Close)
	private, public, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	h := setupConfig(t, func(cfg *config.Config) {
		cfg.VAPIDPublicKey = public
		cfg.VAPIDPrivateKey = private
		cfg.VAPIDSubject = "mailto:test@example.com"
		cfg.PushHTTP = provider.Client()
	})
	a, b := token(t, uuid.New()), token(t, uuid.New())
	p256dh, auth := testPushKeys(t)
	const path = "/v1/me/push-subscriptions"
	body := map[string]any{"endpoint": provider.URL, "keys": map[string]string{"p256dh": p256dh, "auth": auth}}
	save := func(tok string) {
		t.Helper()
		if rec := doJSON(t, h, http.MethodPost, path+"?return_subscription_id=1", tok, body); rec.Code != http.StatusOK {
			t.Fatalf("register: %d", rec.Code)
		}
	}
	send := func(tok string) int { return doJSON(t, h, http.MethodPost, path+"/test", tok, nil).Code }
	save(a)
	if got := send(a); got != http.StatusOK {
		t.Fatalf("first delivery: %d", got)
	}
	if rec := doJSON(t, h, http.MethodDelete, path, a, map[string]string{"endpoint": provider.URL}); rec.Code != http.StatusNoContent {
		t.Fatalf("remove: %d", rec.Code)
	}
	if got := send(a); got != http.StatusUnprocessableEntity || attempts.Load() != 1 {
		t.Fatalf("after removing device: %d, attempts=%d", got, attempts.Load())
	}

	// A の送信が処理中でも、B に登録し直した購読で配信できる。
	save(a)
	reject.Store(http.StatusGone)
	old := make(chan int, 1)
	go func() { old <- send(a) }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("delivery did not reach the provider")
	}
	save(b)
	release <- struct{}{}
	if got := <-old; got != http.StatusUnprocessableEntity {
		t.Fatalf("old delivery: %d", got)
	}
	if got := send(b); got != http.StatusOK {
		t.Fatalf("new delivery: %d", got)
	}

	for _, status := range []int{http.StatusNotFound, http.StatusUnauthorized, http.StatusForbidden} {
		reject.Store(int32(status))
		go func() { old <- send(b) }()
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatal("delivery did not reach the provider")
		}
		if status == http.StatusNotFound {
			save(b)
		}
		release <- struct{}{}
		if got := <-old; got != http.StatusUnprocessableEntity {
			t.Fatalf("provider status %d, old delivery: %d", status, got)
		}
		if got := send(b); got != http.StatusOK {
			t.Fatalf("provider status %d, delivery after recovery: %d", status, got)
		}
	}
	save(a)
	if got := send(a); got != http.StatusOK {
		t.Fatalf("delivery after switching back: %d", got)
	}
	output := logs.String()
	if strings.Contains(output, providerDetail) || strings.Contains(output, provider.URL) {
		t.Fatal("unexpected delivery details in logs")
	}
	if !strings.Contains(output, "status=403 event_id=") {
		t.Fatal("delivery status and event ID were not logged")
	}
}
