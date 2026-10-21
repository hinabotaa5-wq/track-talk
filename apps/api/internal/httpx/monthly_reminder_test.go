package httpx_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMonthlyReminderReachesEveryoneOnce(t *testing.T) {
	var received atomic.Int64
	pushServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusCreated)
	}))
	defer pushServer.Close()
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	fixed := time.Date(2031, 3, 1, 1, 0, 0, 0, time.UTC)
	h := setupConfig(t, func(cfg *config.Config) {
		cfg.VAPIDPublicKey = pub
		cfg.VAPIDPrivateKey = priv
		cfg.VAPIDSubject = "mailto:test@example.com"
		cfg.PushHTTP = pushServer.Client()
		cfg.ReminderSecret = "monthly-secret"
		// コーチングを提供していない間も、通知の登録とお知らせは使える。
		cfg.CoachingEnabled = false
		cfg.Clock = func() time.Time { return fixed }
	})
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `DELETE FROM monthly_reminders WHERE month = '2031-03-01'`); err != nil {
		t.Fatal(err)
	}

	// コーチングを使っていない人も、通知をオンにすれば受け取れる。
	subscribe := func(tok, suffix string) {
		t.Helper()
		p256dh, auth := testPushKeys(t)
		rec := doJSON(t, h, http.MethodPost, "/v1/me/push-subscriptions", tok, map[string]any{
			"endpoint": pushServer.URL + suffix,
			"keys":     map[string]string{"p256dh": p256dh, "auth": auth},
		})
		if rec.Code != http.StatusNoContent {
			t.Fatalf("subscribe %d %s", rec.Code, rec.Body.String())
		}
	}
	a := token(t, uuid.New())
	subscribe(a, "/a-phone")
	subscribe(a, "/a-pc")
	subscribe(token(t, uuid.New()), "/b")
	// 通知をオンにしていない人には送らない。
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", token(t, uuid.New()), nil); rec.Code != http.StatusOK {
		t.Fatalf("profile %d", rec.Code)
	}

	call := func(secret string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/internal/monthly-reminder", nil)
		if secret != "" {
			req.Header.Set("X-Reminder-Secret", secret)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	if rec := call(""); rec.Code != http.StatusNotFound {
		t.Fatalf("no secret %d", rec.Code)
	}
	if rec := call("wrong"); rec.Code != http.StatusNotFound {
		t.Fatalf("wrong secret %d", rec.Code)
	}
	rec := call("monthly-secret")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"recipients":2`) || !strings.Contains(rec.Body.String(), `"month":"2031-03"`) {
		t.Fatalf("send %d %s", rec.Code, rec.Body.String())
	}
	if received.Load() != 3 {
		t.Fatalf("pushes %d", received.Load())
	}
	// Cloud Scheduler が呼び直しても、同じ月に二度は送らない。
	rec = call("monthly-secret")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"already_sent":true`) {
		t.Fatalf("resend %d %s", rec.Code, rec.Body.String())
	}
	if received.Load() != 3 {
		t.Fatalf("pushes after resend %d", received.Load())
	}
}

// Cloud Scheduler の締め切りで接続が切れても、送る権利を取ったあとは全員に送り終える。
func TestMonthlyReminderFinishesAfterCallerDisconnects(t *testing.T) {
	reqCtx, disconnect := context.WithCancel(context.Background())
	defer disconnect()
	var received atomic.Int64
	pushServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 最初の1通が届いたところで、呼び出し元が接続を切る。
		if received.Add(1) == 1 {
			disconnect()
		}
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusCreated)
	}))
	defer pushServer.Close()
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	fixed := time.Date(2031, 4, 1, 1, 0, 0, 0, time.UTC)
	h := setupConfig(t, func(cfg *config.Config) {
		cfg.VAPIDPublicKey = pub
		cfg.VAPIDPrivateKey = priv
		cfg.VAPIDSubject = "mailto:test@example.com"
		cfg.PushHTTP = pushServer.Client()
		cfg.ReminderSecret = "monthly-secret"
		cfg.Clock = func() time.Time { return fixed }
	})
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `DELETE FROM monthly_reminders WHERE month = '2031-04-01'`); err != nil {
		t.Fatal(err)
	}
	// ほかのテストが残した端末にも送るので、数は送る前の登録の数で比べる。
	// IP ごとの書き込みの制限(30回/分)に収まる人数にする。
	const people = 20
	for i := range people {
		p256dh, auth := testPushKeys(t)
		rec := doJSON(t, h, http.MethodPost, "/v1/me/push-subscriptions", token(t, uuid.New()), map[string]any{
			"endpoint": pushServer.URL + "/disconnect-" + uuid.NewString(),
			"keys":     map[string]string{"p256dh": p256dh, "auth": auth},
		})
		if rec.Code != http.StatusNoContent {
			t.Fatalf("subscribe %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	var want int64
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM push_subscriptions WHERE endpoint LIKE $1`, pushServer.URL+"/%").Scan(&want); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/internal/monthly-reminder", nil).WithContext(reqCtx)
	req.Header.Set("X-Reminder-Secret", "monthly-secret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := received.Load(); got != want {
		t.Fatalf("pushes %d, want %d (%d %s)", got, want, rec.Code, rec.Body.String())
	}
}
