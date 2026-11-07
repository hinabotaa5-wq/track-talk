package httpx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	coachRejectedJob    = "coach_rejected"
	coachIdentityEndJob = "coach_identity_end"
)

// 本体と一緒に記録するのは通知の予定だけ。ベルへの書き込みは本体の確定後に行う。
func enqueuePostNotification(ctx context.Context, q *db.Queries, ids *[]uuid.UUID, recipient uuid.UUID, kind string, postID uuid.UUID, commentID, applicationID *uuid.UUID) error {
	id := uuid.New()
	if err := q.EnqueueNotificationJob(ctx, db.EnqueueNotificationJobParams{
		ID: id, Kind: kind, RecipientID: recipient, PostID: &postID, CommentID: commentID, ApplicationID: applicationID,
	}); err != nil {
		return err
	}
	*ids = append(*ids, id)
	return nil
}

func (s *Server) retryNotificationJobs(ctx context.Context) {
	ids, err := s.q.ListDueNotificationJobs(ctx)
	if err != nil {
		slog.Warn("list notification jobs", "error_type", notificationErrorType(err))
		return
	}
	s.runNotificationJobs(ctx, ids)
}

func (s *Server) runNotificationJobs(ctx context.Context, ids []uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	for _, id := range ids {
		if ctx.Err() != nil {
			return
		}
		jobCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		job, err := s.finishNotificationJob(jobCtx, id)
		cancel()
		if err != nil {
			slog.Warn("notification job failed", "job_id", id, "kind", job.Kind, "error_type", notificationErrorType(err))
			retryCtx, retryCancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
			if err := s.q.DeferNotificationJob(retryCtx, id); err != nil {
				slog.Warn("defer notification job failed", "job_id", id, "error_type", notificationErrorType(err))
			}
			retryCancel()
			continue
		}
		if job.Kind == coachRejectedJob {
			msg := coachRejectionMessage()
			msg.Tag = "coach-review-" + job.ID.String()
			s.pushQuietly(ctx, job.RecipientID, notifyCoaching, msg, 24*time.Hour)
		}
	}
}

// ベルと予定の消化を同時に確定する。重なった実行は行ロックで片方だけが進む。
func (s *Server) finishNotificationJob(ctx context.Context, id uuid.UUID) (db.NotificationJob, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return db.NotificationJob{}, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)
	job, err := q.LockNotificationJob(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.NotificationJob{}, nil
	}
	if err != nil {
		return job, err
	}
	if err := writeNotificationJob(ctx, q, job); err != nil {
		return job, err
	}
	if err := q.DeleteNotificationJob(ctx, id); err != nil {
		return job, err
	}
	return job, tx.Commit(ctx)
}

func writeNotificationJob(ctx context.Context, q *db.Queries, job db.NotificationJob) error {
	switch job.Kind {
	case "recruitment_post":
		return q.InsertRecruitmentPostNotifications(ctx, db.InsertRecruitmentPostNotificationsParams{
			PostID: *job.PostID, AuthorID: job.RecipientID,
		})
	case "review_offer":
		return insertReviewNotification(ctx, q, job.RecipientID, job.Kind, *job.ReviewRequestID)
	case coachRejectedJob:
		wanted, err := notificationWanted(ctx, q, job.RecipientID, notifyCoaching, true)
		if err != nil || !wanted {
			return err
		}
		msg := coachRejectionMessage()
		return q.InsertRejectionNotification(ctx, db.InsertRejectionNotificationParams{
			ID: job.ID, RecipientID: job.RecipientID, Title: &msg.Title, Body: &msg.Body, Url: &msg.URL,
		})
	case coachIdentityEndJob:
		_, err := q.LockRejectedCoachApplication(ctx, db.LockRejectedCoachApplicationParams{
			ProfileID: job.RecipientID, ReviewedAt: job.ReviewedAt,
		})
		// 再応募した場合は新しい応募の本人情報を終了扱いにしない。
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		return q.EndCoachLegalIdentity(ctx, job.RecipientID)
	default:
		return insertNotification(ctx, q, job.RecipientID, job.Kind, *job.PostID, job.CommentID, job.ApplicationID)
	}
}

func coachRejectionMessage() push.Message {
	return push.Message{
		Title: "コーチの応募の審査が終わりました",
		Body:  "結果を応募の画面で確認してください",
		URL:   "/coach/apply",
	}
}

func notificationErrorType(err error) string {
	var dbErr *pgconn.PgError
	if errors.As(err, &dbErr) {
		return dbErr.Code
	}
	return fmt.Sprintf("%T", err)
}
