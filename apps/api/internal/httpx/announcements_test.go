package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

// 運営からのお知らせは、管理者だけが送れる。全員のベルと通知をオンにした人の端末に1回だけ届き、
// 確認の入力がないもの・同じ id・同じ文面の送り直しは受け付けない。ベルの分はあとから取り消せる。
func TestAnnouncementReachesEveryoneOnce(t *testing.T) {
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
	h := setupConfig(t, func(cfg *config.Config) {
		cfg.VAPIDPublicKey = pub
		cfg.VAPIDPrivateKey = priv
		cfg.VAPIDSubject = "mailto:test@example.com"
		cfg.PushHTTP = pushServer.Client()
		cfg.CoachingEnabled = false
	})
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

	admin := adminToken(t, h)
	a := token(t, uuid.New())
	subscribe(a, "/a-phone")
	subscribe(a, "/a-pc")
	// お知らせのプッシュをオフにした人には、端末があっても送らない。ベルには入る。
	pushOff := token(t, uuid.New())
	subscribe(pushOff, "/push-off")
	if rec := doJSON(t, h, http.MethodPut, "/v1/me/notification-settings/announcement", pushOff, map[string]any{"in_app": true, "push": false}); rec.Code != http.StatusOK {
		t.Fatalf("mute push %d %s", rec.Code, rec.Body.String())
	}
	// ベルもオフにした人には何も届かない。
	muted := token(t, uuid.New())
	if rec := doJSON(t, h, http.MethodPut, "/v1/me/notification-settings/announcement", muted, map[string]any{"in_app": false, "push": false}); rec.Code != http.StatusOK {
		t.Fatalf("mute all %d %s", rec.Code, rec.Body.String())
	}

	msg := map[string]any{"title": "コーチ機能が始まりました", "body": "パーソナル指導を受けられるようになりました", "url": "/coaches"}

	// 管理者でなければ、人数も見られず、送れない。
	if rec := doJSON(t, h, http.MethodGet, "/v1/admin/announcements/recipients", a, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin recipients %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/announcements", a, withAnnouncement(msg, uuid.NewString(), "送信")); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin send %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/announcements/test", a, msg); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin test %d", rec.Code)
	}

	// 送る前に人数が分かる。ベルは管理者・a・pushOff、プッシュは a だけ。
	rec := doJSON(t, h, http.MethodGet, "/v1/admin/announcements/recipients", admin, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"in_app":3`) || !strings.Contains(rec.Body.String(), `"push":1`) {
		t.Fatalf("recipients %d %s", rec.Code, rec.Body.String())
	}

	// 外のサイトは開けない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/announcements/test", admin, map[string]any{"title": "a", "body": "b", "url": "//evil.example.com"}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("external url %d %s", rec.Code, rec.Body.String())
	}
	// 試し送信は管理者自身のベルにだけ入り、ほかの人には届かない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/announcements/test", admin, msg); rec.Code != http.StatusOK {
		t.Fatalf("test %d %s", rec.Code, rec.Body.String())
	}
	if items, _ := listNotifications(t, h, admin); len(items) != 1 {
		t.Fatalf("admin test bell %+v", items)
	}
	if items, _ := listNotifications(t, h, a); len(items) != 0 || received.Load() != 0 {
		t.Fatalf("test reached others %+v pushes %d", items, received.Load())
	}

	// 「送信」と入力していなければ送らない。
	id := uuid.NewString()
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/announcements", admin, withAnnouncement(msg, id, "")); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("no confirm %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/admin/announcements", admin, withAnnouncement(msg, id, "送信"))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"in_app":3`) || !strings.Contains(rec.Body.String(), `"push_recipients":1`) {
		t.Fatalf("send %d %s", rec.Code, rec.Body.String())
	}
	if received.Load() != 2 {
		t.Fatalf("pushes %d", received.Load())
	}
	for _, tok := range []string{a, pushOff} {
		items, _ := listNotifications(t, h, tok)
		if len(items) != 1 || items[0].Kind != "message" || items[0].URL != "/coaches" {
			t.Fatalf("bell %+v", items)
		}
	}
	if items, _ := listNotifications(t, h, muted); len(items) != 0 {
		t.Fatalf("muted bell %+v", items)
	}

	// 二度押し(同じ id)も、別の id で同じ文面を送り直すのも、受け付けない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/announcements", admin, withAnnouncement(msg, id, "送信")); rec.Code != http.StatusConflict {
		t.Fatalf("same id %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/announcements", admin, withAnnouncement(msg, uuid.NewString(), "送信")); rec.Code != http.StatusConflict {
		t.Fatalf("same body %d %s", rec.Code, rec.Body.String())
	}
	if received.Load() != 2 {
		t.Fatalf("pushes after resend %d", received.Load())
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/admin/announcements", admin, nil)
	var list struct {
		Announcements []struct {
			ID             string `json:"id"`
			InAppCount     int    `json:"in_app_count"`
			PushRecipients int    `json:"push_recipients"`
		} `json:"announcements"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list.Announcements) != 1 || list.Announcements[0].ID != id || list.Announcements[0].InAppCount != 3 || list.Announcements[0].PushRecipients != 1 {
		t.Fatalf("list %d %s", rec.Code, rec.Body.String())
	}

	// 取り消すと、ベルから消える。二度は取り消せない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/announcements/"+id+"/cancel", a, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin cancel %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/announcements/"+id+"/cancel", admin, nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"deleted":3`) {
		t.Fatalf("cancel %d %s", rec.Code, rec.Body.String())
	}
	if items, _ := listNotifications(t, h, a); len(items) != 0 {
		t.Fatalf("bell after cancel %+v", items)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/announcements/"+id+"/cancel", admin, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("cancel twice %d", rec.Code)
	}
	// 取り消したあとは、直した文面を送り直せる。
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/announcements", admin, withAnnouncement(msg, uuid.NewString(), "送信")); rec.Code != http.StatusOK {
		t.Fatalf("resend after cancel %d %s", rec.Code, rec.Body.String())
	}
}

func withAnnouncement(msg map[string]any, id, confirm string) map[string]any {
	out := map[string]any{"id": id, "confirm": confirm}
	for k, v := range msg {
		out[k] = v
	}
	return out
}
