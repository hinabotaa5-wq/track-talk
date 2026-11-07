package httpx

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/jackc/pgx/v5"
)

// 月1回の記録のお知らせ。Cloud Scheduler が毎月1日に呼び、全員のベルと、通知をオンにした人の端末へ送る(本人の設定に従う)。
// 大会に出たかどうかはアプリでは分からないので、全員に同じ文面で送る。
const (
	// 同時に送る数。プッシュサービスへの接続を開きすぎない。
	monthlyReminderWorkers = 8
	// 届かないまま1日たったら、もう出さない(月はじめの話なので遅れて届いても意味が薄い)。
	monthlyReminderTTL = 24 * time.Hour
	// 送り終えるまでの上限。Cloud Scheduler の締め切りで接続が切れても、ここまでは送り続ける。
	monthlyReminderSendTimeout = 30 * time.Minute
)

var monthlyReminderMessage = push.Message{
	Title: "大会の記録を残せていますか？",
	Body:  "先月の大会の記録を残しておくと、自己ベストの推移がグラフで見られます。",
	URL:   "/memo?tab=records",
	Tag:   "monthly-record-reminder",
}

func (s *Server) monthlyReminder(w http.ResponseWriter, r *http.Request) {
	secret := s.cfg.ReminderSecret
	got := r.Header.Get("X-Reminder-Secret")
	if secret == "" || subtle.ConstantTimeCompare([]byte(got), []byte(secret)) != 1 {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	// ベルを開かない人の古い通知も、ここでまとめて消す。補足なので、消せなくてもお知らせは送る。
	if n, err := s.q.DeleteOldNotifications(r.Context()); err != nil {
		slog.Warn("monthly reminder: delete old notifications", "err", err)
	} else if n > 0 {
		slog.Info("monthly reminder: deleted old notifications", "count", n)
	}
	if !s.push.Configured() {
		Error(w, http.StatusServiceUnavailable, "unavailable", "通知はまだ設定されていません")
		return
	}
	// 送る権利を取ったあとは、呼び出し元の接続が切れても最後まで送る。途中で止めると、
	// 今月は送った扱いになって残りの人に二度と届かない。
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), monthlyReminderSendTimeout)
	defer cancel()
	local := s.now().In(jst.Zone)
	month := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, time.UTC)
	claimed := false
	if _, err := s.q.ClaimMonthlyReminder(ctx, month); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			MapDBError(w, err)
			return
		}
		// 送り終えた月か、いま送っている最中の月。止まっていた月の分は下で送り直す。
	} else {
		claimed = true
	}
	var recipients, pending int64
	if claimed {
		var err error
		recipients, pending, err = s.sendMonthly(ctx, month)
		if err != nil {
			MapDBError(w, err)
			return
		}
	}
	// 送り始めたまま止まっている月があれば、未送信分だけ送り直す（本体 #421 F11）。
	if stale, err := s.q.ListStaleMonthlyReminders(ctx); err != nil {
		slog.Warn("monthly reminder: stale", "err", err)
	} else {
		for _, m := range stale {
			if m.Equal(month) {
				continue
			}
			if _, _, err := s.sendMonthly(ctx, m); err != nil {
				slog.Warn("monthly reminder: resume", "month", m.Format("2006-01"), "err", err)
			}
		}
	}
	JSON(w, http.StatusOK, map[string]any{"month": month.Format("2006-01"), "already_sent": !claimed, "recipients": recipients, "pending": pending})
}

// sendMonthly は、その月のベルとプッシュを送る。ベルは1回だけ入れ、プッシュは待ちに入れて送り、
// 途中で止まっても呼び直しで未送信分だけ送り直せる。送った人数と残りの待ち数を返す。
func (s *Server) sendMonthly(ctx context.Context, month time.Time) (int64, int64, error) {
	monthly, _ := findNotificationCategory(notifyMonthly)
	ids, err := s.q.ListPushRecipients(ctx, db.ListPushRecipientsParams{Category: notifyMonthly, Exclude: []uuid.UUID{}, PushDefault: monthly.PushDefault})
	if err != nil {
		// ベルを入れ終える前の失敗だけ、権利を返して呼び直しで送れるようにする。
		// ベルを入れ終えたあとの失敗は権利を残し、呼び直しで残りを送る。
		if row, rerr := s.q.GetMonthlyReminder(ctx, month); rerr != nil || row.InAppDoneAt == nil {
			if rerr := s.q.ReleaseMonthlyReminder(ctx, month); rerr != nil {
				slog.Warn("monthly reminder: release", "err", rerr)
			}
		}
		return 0, 0, err
	}
	// ベルは1回だけ入れる。呼び直しでは入れ直さない。
	if row, err := s.q.GetMonthlyReminder(ctx, month); err != nil {
		return 0, 0, err
	} else if row.InAppDoneAt == nil {
		if err := s.q.InsertMonthlyReminderNotifications(ctx, db.InsertMonthlyReminderNotificationsParams{
			Title: &monthlyReminderMessage.Title,
			Body:  &monthlyReminderMessage.Body,
			Url:   &monthlyReminderMessage.URL,
		}); err != nil {
			slog.Warn("monthly reminder: notifications", "err", err)
		}
		if err := s.q.MarkMonthlyReminderInAppDone(ctx, month); err != nil {
			slog.Warn("monthly reminder: in-app done", "err", err)
		}
	}
	// プッシュは待ちに入れて送る。途中で止まっても、呼び直しで未送信分だけ送り直す。
	prefix := "monthly:" + month.Format("2006-01") + ":"
	expiresAt := s.now().Add(monthlyReminderTTL)
	for _, id := range ids {
		s.enqueuePushOutbox(ctx, id, notifyMonthly, monthlyReminderMessage, monthlyReminderTTL, expiresAt, prefix+id.String())
	}
	s.drainPushOutbox(ctx)
	counts, err := s.q.CountPushOutboxByPrefix(ctx, prefix+"%")
	if err != nil {
		return 0, 0, err
	}
	if counts.Pending == 0 {
		if err := s.q.FinishMonthlyReminder(ctx, db.FinishMonthlyReminderParams{Month: month, Recipients: int32(counts.Sent)}); err != nil {
			slog.Warn("monthly reminder: finish", "err", err)
		}
	}
	return counts.Sent, counts.Pending, nil
}

// sendPushToAll は、ids の人それぞれの端末へ送り、1台以上に届いた人の数を返す。
func (s *Server) sendPushToAll(ctx context.Context, ids []uuid.UUID, msg push.Message, ttl time.Duration) int {
	var reached atomic.Int64
	jobs := make(chan uuid.UUID)
	var wg sync.WaitGroup
	for range monthlyReminderWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				sent, err := s.sendPush(ctx, id, msg, ttl)
				if err != nil {
					slog.Warn("push: list subscriptions", "err", err)
					continue
				}
				if sent > 0 {
					reached.Add(1)
				}
			}
		}()
	}
	for _, id := range ids {
		jobs <- id
	}
	close(jobs)
	wg.Wait()
	return int(reached.Load())
}
