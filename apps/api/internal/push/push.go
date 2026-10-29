// Package push は Web プッシュ通知（VAPID）を送る。
package push

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

var (
	ErrNotConfigured = errors.New("web push not configured")
	// ErrGone は宛先がもう使えない（購読が解除された、ブラウザのデータが消えた）ことを表す。宛先は消してよい。
	ErrGone   = errors.New("push subscription gone")
	ErrFailed = errors.New("push failed")
)

type Sender struct {
	PublicKey  string
	PrivateKey string
	// Subject は VAPID の連絡先（mailto: か https:）。プッシュの配信元が問題を連絡するときに使う。
	Subject string
	HTTP    webpush.HTTPClient
}

func (s Sender) Configured() bool {
	return s.PublicKey != "" && s.PrivateKey != "" && s.Subject != ""
}

type Subscription struct {
	Endpoint string
	P256dh   string
	Auth     string
}

// Message は Service Worker が受け取って通知として表示する中身。
type Message struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	// URL は通知をタップしたときに開くサイト内のパス。
	URL string `json:"url"`
	// Tag が同じ通知は、端末上で1つにまとまる。
	Tag string `json:"tag,omitempty"`
}

// Send は1つの宛先に送る。宛先が使えなくなっていたら ErrGone を返す。
func (s Sender) Send(ctx context.Context, sub Subscription, msg Message, ttl time.Duration) error {
	if !s.Configured() {
		return ErrNotConfigured
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	client := s.HTTP
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := webpush.SendNotificationWithContext(ctx, payload, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys:     webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
	}, &webpush.Options{
		HTTPClient:      client,
		Subscriber:      s.Subject,
		TTL:             int(ttl.Seconds()),
		Urgency:         webpush.UrgencyHigh,
		VAPIDPublicKey:  s.PublicKey,
		VAPIDPrivateKey: s.PrivateKey,
	})
	if err != nil {
		// url.Error は宛先の URL を含む。宛先は知っていれば送れてしまうので、ログに残さない。
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return fmt.Errorf("%w: %v", ErrFailed, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return ErrGone
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		// VAPID の鍵を入れ替えたあと、前の鍵で作った購読はずっと拒否される（Apple の BadJwtToken、
		// Chrome の「credentials do not correspond」）。残しても届かないので、宛先を消す。端末は次に
		// アプリを開いたとき、いまの鍵で購読を作り直して登録し直す。
		return fmt.Errorf("%w: status %d: %s", ErrGone, resp.StatusCode, failureReason(body))
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		// 失敗の理由（Apple の BadJwtToken など）を、原因を調べられるように残す。
		return fmt.Errorf("%w: status %d: %s", ErrFailed, resp.StatusCode, failureReason(body))
	}
	return nil
}

// failureReason は、プッシュサービスが返した失敗の理由を、ログに残せる短さにする。
func failureReason(body []byte) string {
	reason := strings.Join(strings.Fields(string(body)), " ")
	if len(reason) > 200 {
		reason = reason[:200]
	}
	return reason
}
