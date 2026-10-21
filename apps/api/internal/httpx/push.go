package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/push"
)

// pushPublicKey は、ブラウザがプッシュの購読を作るときに使う VAPID の公開鍵を返す。
func (s *Server) pushPublicKey(w http.ResponseWriter, r *http.Request) {
	if !s.push.Configured() {
		Error(w, http.StatusServiceUnavailable, "unavailable", "通知はまだ設定されていません")
		return
	}
	JSON(w, http.StatusOK, map[string]any{"public_key": s.push.PublicKey})
}

type pushSubscriptionReq struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
	// ブラウザの PushSubscription.toJSON() にそのまま付いてくる項目。使わない。
	ExpirationTime *int64 `json:"expirationTime"`
}

// pushServiceHosts は、ブラウザのプッシュサービスのホスト。宛先はここだけに限り、
// 任意の URL を登録させて API から内部のアドレスへ接続させる（SSRF）ことを防ぐ。
var pushServiceHosts = []string{
	"fcm.googleapis.com",                // Chrome・Edge(Chromium)・Android・Samsung Internet・Opera
	"updates.push.services.mozilla.com", // Firefox
	"web.push.apple.com",                // Safari・iOS のホーム画面のアプリ
	".push.apple.com",
	".notify.windows.com", // 古い Edge
}

func pushServiceHost(host string) bool {
	host = strings.ToLower(host)
	for _, h := range pushServiceHosts {
		if host == h || (strings.HasPrefix(h, ".") && strings.HasSuffix(host, h)) {
			return true
		}
	}
	return false
}

func (s *Server) validPushEndpoint(raw string) bool {
	if len(raw) > 1000 {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return false
	}
	// テストで送信先を差し替えたときは、そのテスト用サーバーへの登録を許す。
	if s.cfg.PushHTTP != nil {
		return true
	}
	return u.Port() == "" && pushServiceHost(u.Hostname())
}

func (s *Server) savePushSubscription(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var req pushSubscriptionReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	p256dh := strings.TrimSpace(req.Keys.P256dh)
	auth := strings.TrimSpace(req.Keys.Auth)
	if !s.validPushEndpoint(req.Endpoint) || p256dh == "" || len(p256dh) > 200 || auth == "" || len(auth) > 100 {
		Error(w, http.StatusUnprocessableEntity, "validation_error", "通知の登録内容が不正です")
		return
	}
	if _, err := s.q.UpsertPushSubscription(r.Context(), db.UpsertPushSubscriptionParams{
		ID:        uuid.New(),
		ProfileID: p.ID,
		Endpoint:  req.Endpoint,
		P256dh:    p256dh,
		Auth:      auth,
	}); err != nil {
		MapDBError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type pushEndpointReq struct {
	Endpoint string `json:"endpoint"`
}

func (s *Server) deletePushSubscription(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var req pushEndpointReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if _, err := s.q.DeletePushSubscription(r.Context(), db.DeletePushSubscriptionParams{
		ProfileID: p.ID,
		Endpoint:  req.Endpoint,
	}); err != nil {
		MapDBError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// testPush は、自分の端末に試しの通知を送る。通知が届くかを本人が確かめるためのもの。
func (s *Server) testPush(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	if !s.push.Configured() {
		Error(w, http.StatusServiceUnavailable, "unavailable", "通知はまだ設定されていません")
		return
	}
	sent, err := s.sendPush(r.Context(), p.ID, push.Message{
		Title: "TrackTalk",
		Body:  "通知の設定ができました",
		URL:   "/me/push",
		Tag:   "push-test",
	}, 10*time.Minute)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if sent == 0 {
		Error(w, http.StatusUnprocessableEntity, "validation_error", "通知を受け取れる端末がありません。この端末で通知をオンにしてください")
		return
	}
	JSON(w, http.StatusOK, map[string]any{"sent": sent})
}

// sendPush は、その人が登録したすべての端末に送る。使えなくなった宛先は消す。
// 送れた端末の数を返す。送信の失敗は記録だけして、呼び出し元の処理は止めない。
func (s *Server) sendPush(ctx context.Context, profileID uuid.UUID, msg push.Message, ttl time.Duration) (int, error) {
	subs, err := s.q.ListPushSubscriptions(ctx, profileID)
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, sub := range subs {
		// 宛先を限る前に登録されたものも、プッシュサービス以外には送らない。
		if !s.validPushEndpoint(sub.Endpoint) {
			continue
		}
		err := s.push.Send(ctx, push.Subscription{Endpoint: sub.Endpoint, P256dh: sub.P256dh, Auth: sub.Auth}, msg, ttl)
		switch {
		case err == nil:
			sent++
		case errors.Is(err, push.ErrGone):
			if err := s.q.DeletePushSubscriptionByEndpoint(ctx, sub.Endpoint); err != nil {
				slog.Warn("push: delete gone subscription", "err", err)
			}
		default:
			slog.Warn("push: send failed", "err", err)
		}
	}
	return sent, nil
}
