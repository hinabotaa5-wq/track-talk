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

// 月1回の記録のお知らせ。Cloud Scheduler が毎月1日に呼び、通知をオンにしたすべての人の端末へ送る。
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
	Title: "大会の記録、残せていますか？",
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
	if _, err := s.q.ClaimMonthlyReminder(ctx, month); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// 呼び直し。今月はもう送った。
			JSON(w, http.StatusOK, map[string]any{"month": month.Format("2006-01"), "already_sent": true, "recipients": 0})
			return
		}
		MapDBError(w, err)
		return
	}
	ids, err := s.q.ListPushProfileIDs(ctx)
	if err != nil {
		// まだ誰にも送っていないので、権利を返して呼び直しで送れるようにする。
		if err := s.q.ReleaseMonthlyReminder(ctx, month); err != nil {
			slog.Warn("monthly reminder: release", "err", err)
		}
		MapDBError(w, err)
		return
	}
	recipients := s.sendPushToAll(ctx, ids, monthlyReminderMessage, monthlyReminderTTL)
	if err := s.q.FinishMonthlyReminder(ctx, db.FinishMonthlyReminderParams{Month: month, Recipients: int32(recipients)}); err != nil {
		slog.Warn("monthly reminder: finish", "err", err)
	}
	JSON(w, http.StatusOK, map[string]any{"month": month.Format("2006-01"), "already_sent": false, "recipients": recipients})
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
