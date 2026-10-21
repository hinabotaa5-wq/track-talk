package httpx_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

// 経過報告のプッシュは、報告した人を待たせずに裏で送る。「わかる」や保存をした人が多いと、
// 1人ずつ送り終えるまで報告の画面が止まってしまう。
func TestInjuryUpdatePushesDoNotBlockTheReport(t *testing.T) {
	const delay = 300 * time.Millisecond
	var pushes atomic.Int32
	pushServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		pushes.Add(1)
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
	author := tokenWithProfile(t, uuid.New(), "報告する人", true)
	rec := postInjury(t, h, author, map[string]any{
		"kind": "injury", "title": "プッシュの確認", "body": "本文", "body_part": "achilles-tendinitis",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var post struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &post); err != nil {
		t.Fatal(err)
	}
	const followers = 5
	for i := 0; i < followers; i++ {
		tok := tokenWithProfile(t, uuid.New(), fmt.Sprintf("見守り%d", i), true)
		joinGroup(t, h, tok, "achilles-tendinitis")
		if r := doJSON(t, h, http.MethodPost, "/v1/posts/"+post.ID+"/likes", tok, nil); r.Code >= 300 {
			t.Fatalf("like %d %s", r.Code, r.Body.String())
		}
		p256dh, auth := testPushKeys(t)
		if r := doJSON(t, h, http.MethodPost, "/v1/me/push-subscriptions", tok, map[string]any{
			"endpoint": fmt.Sprintf("%s/f%d", pushServer.URL, i),
			"keys":     map[string]string{"p256dh": p256dh, "auth": auth},
		}); r.Code != http.StatusNoContent {
			t.Fatalf("subscribe %d %s", r.Code, r.Body.String())
		}
	}

	start := time.Now()
	r := doJSON(t, h, http.MethodPost, "/v1/posts/"+post.ID+"/injury/updates", author, map[string]any{"stage": "rehab", "body": "歩けた"})
	took := time.Since(start)
	if r.Code != http.StatusCreated {
		t.Fatalf("update %d %s", r.Code, r.Body.String())
	}
	if took >= 2*delay {
		t.Fatalf("report waited %s for %d pushes", took, followers)
	}
	// 裏で送ったプッシュは、すべて届く。
	deadline := time.Now().Add(10 * time.Second)
	for pushes.Load() < followers && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if got := pushes.Load(); got != followers {
		t.Fatalf("pushes = %d, want %d", got, followers)
	}
}
