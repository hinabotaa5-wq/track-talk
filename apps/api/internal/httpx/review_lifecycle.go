package httpx

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/jackc/pgx/v5"
)

func (s *Server) startReviewWork(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireApprovedReviewCoach(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	rr, err := s.q.GetCoachReviewRequest(ctx, db.GetCoachReviewRequestParams{ID: id, CoachID: p.ID})
	if err != nil {
		MapDBError(w, err)
		return
	}
	_, err = s.q.StartReviewWork(ctx, db.StartReviewWorkParams{ID: rr.ID, CoachID: p.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "conflict", "この依頼の添削は開始できません")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	s.getCoachReviewRequest(w, r)
}

func (s *Server) cancelUnstartedReview(w http.ResponseWriter, r *http.Request) {
	rr, ok := s.requesterReview(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	updated, err := s.q.CancelUnstartedReview(ctx, db.CancelUnstartedReviewParams{ID: rr.ID, RequesterID: rr.RequesterID})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "conflict", "この依頼はいまキャンセルできません")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	if updated.CoachID != nil {
		s.notify(ctx, *updated.CoachID, notifyImportant, push.Message{
			Title: "選手が添削依頼をキャンセルしました",
			URL:   "/coach/reviewing",
			Tag:   "review-canceled-" + rr.ID.String(),
		}, 24*time.Hour)
	}
	s.writeRequesterReview(w, r, updated)
}

// Close overdue work and pause the affected coaches before the next application can be accepted.
func (s *Server) expireOverdueReviews(ctx context.Context) ([]db.ExpireOverdueReviewRequestsRow, error) {
	var expired []db.ExpireOverdueReviewRequestsRow
	err := s.tx(ctx, func(q *db.Queries) error {
		if err := q.LockReviewAssignment(ctx); err != nil {
			return err
		}
		var err error
		expired, err = q.ExpireOverdueReviewRequests(ctx)
		if err != nil {
			return err
		}
		coaches := map[uuid.UUID]bool{}
		newTimeouts := map[uuid.UUID]bool{}
		for _, rr := range expired {
			if rr.CoachID != nil && rr.OldStatus == "in_review" {
				coaches[*rr.CoachID] = true
				newTimeouts[rr.ID] = true
			}
		}
		for coachID := range coaches {
			if err := q.PauseCoachAfterReviewTimeout(ctx, coachID); err != nil {
				return err
			}
			if err := alertReviewTimeoutStreak(ctx, q, coachID, newTimeouts); err != nil {
				return err
			}
		}
		return nil
	})
	return expired, err
}

func alertReviewTimeoutStreak(ctx context.Context, q *db.Queries, coachID uuid.UUID, newTimeouts map[uuid.UUID]bool) error {
	rows, err := q.ListRecentReviewOutcomes(ctx, &coachID)
	if err != nil {
		return err
	}
	if len(rows) < 3 || rows[0].TimedOutAt == nil || rows[1].TimedOutAt == nil || rows[2].TimedOutAt == nil {
		return nil
	}
	if !newTimeouts[rows[0].ID] || (len(rows) == 4 && rows[3].TimedOutAt != nil && !newTimeouts[rows[1].ID] && !newTimeouts[rows[2].ID]) {
		return nil
	}
	ids, err := q.ListAdminProfileIDs(ctx)
	if err != nil {
		return err
	}
	title := "フォーム添削が3回連続で期限切れになりました"
	body := "コーチID " + coachID.String()
	url := "/admin"
	for _, id := range ids {
		if err := q.InsertMessageNotification(ctx, db.InsertMessageNotificationParams{
			ID: uuid.New(), RecipientID: id, Title: &title, Body: &body, Url: &url,
		}); err != nil {
			return err
		}
	}
	return nil
}
