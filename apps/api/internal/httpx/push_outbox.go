package httpx

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/push"
)

// 送るべき Push の待ち行列（本体 #421 F04・F11）。一時障害で送れなくても静かに失わず、送り直す。
// 待ちは依頼・配信単位で1行。送るたびにいまの購読と設定を確かめるので、購読が消えていた端末に
// 後から購読が戻っても、次の送り直しで届く。同じ出来事の二重送信は dedup_key で防ぐ。

// pushOutboxBatch は、1回の sweep で送る待ちの上限。打診・返却は件数が少ないので十分に足りる。
const pushOutboxBatch = 100

// last_result に残す名前。送信失敗・購読なし・プロバイダ受理を区別して追えるようにする。
const (
	pushOutboxSent           = "sent"
	pushOutboxNoSubscription = "no_subscription"
	pushOutboxGone           = "gone"
	pushOutboxFailed         = "failed"
	pushOutboxOptedOut       = "opted_out"
)

// enqueuePushOutbox は、送るべき Push を1件入れる。同じ出来事は1行だけで、二重に入れても増えない。
// 設定で切っている人・通知が未設定の環境には入れない。
func (s *Server) enqueuePushOutbox(ctx context.Context, to uuid.UUID, category string, msg push.Message, ttl time.Duration, expiresAt time.Time, dedupKey string) {
	if !s.push.Configured() {
		return
	}
	if ok, err := notificationWanted(ctx, s.q, to, category, false); err != nil || !ok {
		if err != nil {
			slog.Warn("push outbox: notification setting", "err", err)
		}
		return
	}
	if err := s.q.EnqueuePushOutbox(ctx, db.EnqueuePushOutboxParams{
		ID:         uuid.New(),
		ProfileID:  to,
		Title:      msg.Title,
		Body:       msg.Body,
		Url:        msg.URL,
		Tag:        msg.Tag,
		Category:   category,
		TtlSeconds: int32(ttl.Seconds()),
		DedupKey:   dedupKey,
		ExpiresAt:  expiresAt,
	}); err != nil {
		slog.Warn("push outbox: enqueue", "err", err)
	}
}

// queuePush は、送るべき Push を入れて、その場で1回送る。送れなかった分は sweep が送り直す。
// 受け入れ先は打診と返却だけで、それ以外は既存の pushQuietly のまま（範囲は報告に書く）。
func (s *Server) queuePush(ctx context.Context, to uuid.UUID, category string, msg push.Message, ttl time.Duration, expiresAt time.Time, dedupKey string) {
	s.enqueuePushOutbox(ctx, to, category, msg, ttl, expiresAt, dedupKey)
	s.sendPendingOutbox(ctx, 1)
}

// sendPendingOutbox は、送りどきの待ちを古い順に送る。送れた待ちの数を返す。
// 期限切れの待ちは終わりにし、終わって古い行は消す。呼び出し側は tick と一斉配信。
func (s *Server) sendPendingOutbox(ctx context.Context, limit int32) int {
	if !s.push.Configured() {
		return 0
	}
	if _, err := s.q.ExpirePushOutbox(ctx); err != nil {
		slog.Warn("push outbox: expire", "err", err)
	}
	if err := s.q.PurgePushOutbox(ctx); err != nil {
		slog.Warn("push outbox: purge", "err", err)
	}
	rows, err := s.q.ClaimPendingPushOutbox(ctx, limit)
	if err != nil {
		slog.Warn("push outbox: claim", "err", err)
		return 0
	}
	sent := 0
	for _, row := range rows {
		if s.attemptOutboxRow(ctx, row) {
			sent++
		}
	}
	return sent
}

// drainPushOutbox は、送りどきの待ちがなくなるまで送る。一斉配信の呼び出しの中で使う。
// 待ち時間になった待ち（指数バックオフ中）は送らず置いておく。送り直しは tick と呼び直しが担う。
func (s *Server) drainPushOutbox(ctx context.Context) {
	for s.sendPendingOutbox(ctx, pushOutboxBatch) > 0 {
	}
}

// attemptOutboxRow は、待ちを1件送る。送るたびにいまの設定を確かめ、途中で切った人には送り直さない。
// 送れたかどうかを返す。
func (s *Server) attemptOutboxRow(ctx context.Context, row db.ClaimPendingPushOutboxRow) bool {
	if ok, err := notificationWanted(ctx, s.q, row.ProfileID, row.Category, false); err != nil || !ok {
		if err != nil {
			slog.Warn("push outbox: notification setting", "err", err)
			s.failOutboxRow(ctx, row.ID, pushOutboxFailed)
			return false
		}
		// 待ちに入れたあとで設定を切った人。送り直さず終わりにする。
		discarded := pushOutboxOptedOut
		if err := s.q.MarkPushOutboxDiscarded(ctx, db.MarkPushOutboxDiscardedParams{Result: &discarded, ID: row.ID}); err != nil {
			slog.Warn("push outbox: discard", "err", err)
			s.failOutboxRow(ctx, row.ID, pushOutboxOptedOut)
			return false
		}
		return false
	}
	msg := push.Message{Title: row.Title, Body: row.Body, URL: row.Url, Tag: row.Tag}
	sent, subs, gone, err := s.sendToSubscriptions(ctx, row.ProfileID, msg, time.Duration(row.TtlSeconds)*time.Second)
	if err != nil {
		slog.Warn("push outbox: subscriptions", "err", err)
		s.failOutboxRow(ctx, row.ID, pushOutboxFailed)
		return false
	}
	switch {
	case sent > 0:
		if err := s.q.MarkPushOutboxSent(ctx, row.ID); err != nil {
			slog.Warn("push outbox: sent", "err", err)
			s.failOutboxRow(ctx, row.ID, pushOutboxFailed)
			return false
		}
		return true
	case subs == 0:
		// 購読がない。消えた購読が戻れば次の送り直しで届く。
		s.failOutboxRow(ctx, row.ID, pushOutboxNoSubscription)
		return false
	case gone == subs:
		s.failOutboxRow(ctx, row.ID, pushOutboxGone)
		return false
	default:
		s.failOutboxRow(ctx, row.ID, pushOutboxFailed)
		return false
	}
}

func (s *Server) failOutboxRow(ctx context.Context, id uuid.UUID, result string) {
	if err := s.q.MarkPushOutboxFailed(ctx, db.MarkPushOutboxFailedParams{Result: &result, ID: id}); err != nil {
		slog.Warn("push outbox: retry", "err", err)
	}
}

// sendToSubscriptions は、その人のいまの購読すべてに送る。届いた端末数、購読数、使えなくなっていた宛先数、購読の読み出し失敗を返す。
// 使えなくなっていた宛先は消す。送信の一時失敗は数えるだけで、呼び出し側が送り直すかどうかを決める。
func (s *Server) sendToSubscriptions(ctx context.Context, profileID uuid.UUID, msg push.Message, ttl time.Duration) (sent, subs, gone int, err error) {
	subscriptions, err := s.q.ListPushSubscriptions(ctx, profileID)
	if err != nil {
		return 0, 0, 0, err
	}
	for _, sub := range subscriptions {
		// 宛先を限る前に登録されたものも、プッシュサービス以外には送らない。
		if !s.validPushEndpoint(sub.Endpoint) {
			continue
		}
		subs++
		sendErr := s.push.Send(ctx, push.Subscription{Endpoint: sub.Endpoint, P256dh: sub.P256dh, Auth: sub.Auth}, msg, ttl)
		switch {
		case sendErr == nil:
			sent++
		case isPushGone(sendErr):
			gone++
			if sendErr != push.ErrGone {
				// 401・403 で拒否された宛先を消すときは、理由を残す。設定の誤りで消えていないかを確かめられるように。
				slog.Warn("push: remove rejected subscription", "err", sendErr)
			}
			if err := s.q.DeletePushSubscriptionByEndpoint(ctx, sub.Endpoint); err != nil {
				slog.Warn("push: delete gone subscription", "err", err)
			}
		default:
			slog.Warn("push: send failed", "err", sendErr)
		}
	}
	return sent, subs, gone, nil
}

func isPushGone(err error) bool {
	return errors.Is(err, push.ErrGone)
}
