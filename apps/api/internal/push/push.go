// Package push は Web プッシュ通知（VAPID）を送る。
package push

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

var (
	ErrNotConfigured = errors.New("web push not configured")
	ErrGone          = errors.New("push subscription gone")
	ErrRejected      = errors.New("push authorization rejected")
	ErrFailed        = errors.New("push failed")
)

// DeliveryError contains only a fixed classification and the HTTP status.
type DeliveryError struct {
	Kind   error
	Status int
}

func (e *DeliveryError) Error() string {
	return fmt.Sprintf("%s: status %d", e.Kind, e.Status)
}

func (e *DeliveryError) Unwrap() error { return e.Kind }

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
	Tag            string `json:"tag,omitempty"`
	SubscriptionID string `json:"subscription_id,omitempty"`
}

// Send は1つの宛先に送る。宛先が使えなくなっていたら ErrGone を返す。
func (s Sender) Send(ctx context.Context, sub Subscription, msg Message, ttl time.Duration) error {
	if !s.Configured() {
		return ErrNotConfigured
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return ErrFailed
	}
	client := s.HTTP
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := webpush.SendNotificationWithContext(ctx, payload, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys:     webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
	}, &webpush.Options{
		HTTPClient: client,
		// webpush-go は https: で始まらない宛先に自分で mailto: を付ける。設定に mailto: が付いていると
		// mailto:mailto: になり、Apple に BadJwtToken で拒否されるので、外してから渡す。
		Subscriber:      strings.TrimPrefix(s.Subject, "mailto:"),
		TTL:             int(ttl.Seconds()),
		Urgency:         webpush.UrgencyHigh,
		VAPIDPublicKey:  s.PublicKey,
		VAPIDPrivateKey: s.PrivateKey,
	})
	if err != nil {
		return ErrFailed
	}
	defer resp.Body.Close()
	status := resp.StatusCode
	switch {
	case status == http.StatusNotFound || status == http.StatusGone:
		return &DeliveryError{Kind: ErrGone, Status: status}
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return &DeliveryError{Kind: ErrRejected, Status: status}
	case status < 200 || status >= 300:
		return &DeliveryError{Kind: ErrFailed, Status: status}
	}
	return nil
}
