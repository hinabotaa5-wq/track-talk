package httpx_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

// 1人が登録できる端末は10台まで。超えたら古いものから外し、テスト送信も10台にしか送らない。
func TestPushSubscriptionsAreCappedPerPerson(t *testing.T) {
	var mu sync.Mutex
	received := map[string]int{}
	pushServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		received[r.URL.Path]++
		mu.Unlock()
		w.WriteHeader(http.StatusCreated)
	}))
	defer pushServer.Close()
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	h := setupConfig(t, func(cfg *config.Config) {
		cfg.VAPIDPublicKey = pub
		cfg.VAPIDPrivateKey = priv
		cfg.VAPIDSubject = "mailto:test@example.com"
		cfg.PushHTTP = pushServer.Client()
		cfg.CoachingEnabled = false
	})

	tok := token(t, uuid.New())
	for i := range 12 {
		p256dh, auth := testPushKeys(t)
		rec := doJSON(t, h, http.MethodPost, "/v1/me/push-subscriptions", tok, map[string]any{
			"endpoint": fmt.Sprintf("%s/device-%02d", pushServer.URL, i),
			"keys":     map[string]string{"p256dh": p256dh, "auth": auth},
		})
		if rec.Code != http.StatusNoContent {
			t.Fatalf("subscribe %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	// 別の人の端末は外さない。
	other := token(t, uuid.New())
	p256dh, auth := testPushKeys(t)
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/push-subscriptions", other, map[string]any{
		"endpoint": pushServer.URL + "/other",
		"keys":     map[string]string{"p256dh": p256dh, "auth": auth},
	}); rec.Code != http.StatusNoContent {
		t.Fatalf("other subscribe %d", rec.Code)
	}

	rec := doJSON(t, h, http.MethodPost, "/v1/me/push-subscriptions/test", tok, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"sent":10`) {
		t.Fatalf("test push %d %s", rec.Code, rec.Body.String())
	}
	mu.Lock()
	oldest, newest := received["/device-00"]+received["/device-01"], received["/device-11"]
	mu.Unlock()
	if oldest != 0 {
		t.Fatalf("oldest devices still received %d pushes", oldest)
	}
	if newest != 1 {
		t.Fatalf("newest device got %d", newest)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/me/push-subscriptions/test", other, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"sent":1`) {
		t.Fatalf("other test push %d %s", rec.Code, rec.Body.String())
	}
}
