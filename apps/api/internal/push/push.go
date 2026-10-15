// Package push は Web プッシュ通知（VAPID）を送る。
package push

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
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
		return ErrFailed
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return ErrGone
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return ErrFailed
	}
	return nil
}
