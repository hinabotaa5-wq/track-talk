package httpx

import (
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/stripe"
)

// stripeConnectWebhook は口座の状態同期専用。公開フラグに関係なく、Connect の署名だけを認める。
// 通常の決済 Webhook をここへ向けても、購入成立や送金は実行しない。
func (s *Server) stripeConnectWebhook(w http.ResponseWriter, r *http.Request) {
	if !s.stripe.Configured() || s.cfg.StripeConnectWebhookSecret == "" {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "読めませんでした")
		return
	}
	ev, err := stripe.VerifyWebhook(payload, r.Header.Get("Stripe-Signature"), s.cfg.StripeConnectWebhookSecret, time.Now())
	if err != nil {
		Error(w, http.StatusBadRequest, "bad_signature", "署名が正しくありません")
		return
	}
	if ev.Type != "account.updated" {
		Error(w, http.StatusBadRequest, "unsupported_event", "この通知は受け付けていません")
		return
	}
	if err := s.syncCoachAccount(r.Context(), ev); err != nil {
		slog.Error("stripe connect webhook", "event", ev.ID, "err", err)
		Error(w, http.StatusInternalServerError, "internal_error", "処理できませんでした")
		return
	}
	JSON(w, http.StatusOK, map[string]any{"received": true})
}
