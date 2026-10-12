package httpx

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
)

const withdrawConfirm = "退会する"

type deleteAccountReq struct {
	Confirm string `json:"confirm"`
}

func (s *Server) deleteAccount(w http.ResponseWriter, r *http.Request) {
	uid, ok := UserID(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "unauthorized", "ログインが必要です")
		return
	}
	var req deleteAccountReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if req.Confirm != withdrawConfirm {
		FieldError(w, map[string]string{"confirm": "確認のため「退会する」と入力してください"})
		return
	}

	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())

	if err := purgeAccount(r.Context(), s.q.WithTx(tx), uid); err != nil {
		MapDBError(w, err)
		return
	}

	authDeleted := false
	if s.authAdmin.Enabled() {
		if err := s.authAdmin.DeleteUser(r.Context(), uid); err != nil {
			slog.Error("delete auth user", "err", err, "user_id", uid.String())
			Error(w, http.StatusBadGateway, "auth_delete_failed", "退会を完了できませんでした。時間をおいてもう一度試してください")
			return
		}
		authDeleted = true
	} else {
		slog.Warn("supabase service role key is not set; auth user was not deleted", "user_id", uid.String())
	}

	if err := tx.Commit(r.Context()); err != nil {
		slog.Error("account delete commit failed", "err", err, "user_id", uid.String(), "auth_deleted", authDeleted)
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]bool{"auth_deleted": authDeleted})
}

func purgeAccount(ctx context.Context, q *db.Queries, userID uuid.UUID) error {
	steps := []func(context.Context, uuid.UUID) error{
		q.ReleaseCommentThreadsForUser,
		q.DecrementCommentCountsForUser,
		q.DecrementLikeCountsForUser,
		q.DeletePracticeRatingsForUser,
		q.DeletePracticeApplicationsForUser,
		q.DeleteReportsForUser,
		q.DeleteCommentsForUser,
		q.DeletePostsForUser,
		q.DeleteProfile,
	}
	for _, step := range steps {
		if err := step(ctx, userID); err != nil {
			return err
		}
	}
	return nil
}
