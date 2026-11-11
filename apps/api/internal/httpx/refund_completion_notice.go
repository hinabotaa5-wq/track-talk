package httpx

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/hinata/track-talk/apps/api/internal/stripe"
	"github.com/jackc/pgx/v5"
)

func enqueueConsultationRefundNotice(ctx context.Context, q *db.Queries, operationID uuid.UUID, c db.CoachingConsultation) error {
	if c.RequesterID == nil {
		return nil
	}
	return q.EnqueueConsultationRefundNotice(ctx, db.EnqueueConsultationRefundNoticeParams{
		ID: operationID, RecipientID: *c.RequesterID, Url: consultationNoticePath(c, false),
	})
}

// 既存の返金イベントで、保留中の自動返金通知だけを確定させる。
func resolvePendingReviewRefundNotice(ctx context.Context, q *db.Queries, remote stripe.Refund) (uuid.UUID, error) {
	if remote.Amount <= 0 || remote.Amount > math.MaxInt32 {
		return uuid.Nil, nil
	}
	params := db.ConfirmPendingReviewRefundNoticeParams{
		StripeRefundID: &remote.ID, PaymentIntentID: &remote.PaymentIntent, AmountYen: int32(remote.Amount),
	}
	switch remote.Status {
	case "succeeded":
		id, err := q.ConfirmPendingReviewRefundNotice(ctx, params)
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, nil
		}
		return id, err
	case "failed", "canceled":
		return uuid.Nil, q.CancelPendingReviewRefundNotice(ctx, db.CancelPendingReviewRefundNoticeParams(params))
	}
	return uuid.Nil, nil
}

func (s *Server) retryRefundCompletionNotices(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	ids, err := s.q.ListDueRefundCompletionNotices(ctx)
	if err != nil {
		slog.Warn("list refund completion notices failed")
		return
	}
	for _, id := range ids {
		if ctx.Err() != nil {
			return
		}
		s.deliverRefundCompletionNotice(ctx, id)
	}
}

func (s *Server) deliverRefundCompletionNotice(ctx context.Context, id uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	var recipient uuid.UUID
	var message push.Message
	var inserted int64
	err := s.tx(ctx, func(q *db.Queries) error {
		notice, err := q.LockRefundCompletionNotice(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		recipient = notice.RecipientID
		message = refundCompletionMessage(notice)
		// 一時ミュート中は、許可リスト外の宛先にはベルを作らず予定だけ消す。返金自体は済んでいる。
		if s.coachingMutedFor(recipient) {
			return q.DeleteRefundCompletionNotice(ctx, id)
		}
		inserted, err = q.InsertRefundCompletionBell(ctx, db.InsertRefundCompletionBellParams{
			ID: id, RecipientID: recipient, Title: &message.Title, Body: &message.Body, Url: &message.URL,
		})
		if err != nil {
			return err
		}
		return q.DeleteRefundCompletionNotice(ctx, id)
	})
	if err != nil {
		slog.Warn("refund completion notice failed", "notice_id", id)
		retryCtx, retryCancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer retryCancel()
		if err := s.q.DeferRefundCompletionNotice(retryCtx, id); err != nil {
			slog.Warn("defer refund completion notice failed", "notice_id", id)
		}
		return
	}
	// ベルと送信予定を同時に確定した処理だけが、設定に関係なく Push を送る。
	if inserted == 1 {
		s.pushQuietly(ctx, recipient, notifyImportant, message, 24*time.Hour)
	}
}

func refundCompletionMessage(notice db.RefundCompletionNotice) push.Message {
	message := push.Message{
		Title: "フォーム添削の返金が完了しました",
		Body:  "お支払いの状況は依頼画面で確認できます",
		URL:   notice.Url,
		Tag:   "refund-complete-" + notice.ID.String(),
	}
	switch notice.Kind {
	case "review_cancel":
		message.Title = "フォーム添削の仮押さえの取り消しが完了しました"
		message.Body = "カードの仮押さえを取り消しました"
	case "consultation_refund":
		message.Title = "追加相談の返金が完了しました"
		message.Body = "お支払いの状況は追加相談の画面で確認できます"
	}
	return message
}
