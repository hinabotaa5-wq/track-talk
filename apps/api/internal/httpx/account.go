package httpx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
)

var errAuthDelete = errors.New("auth delete")

const withdrawConfirm = "退会する"
const withdrawMaxLoginAge = 5 * time.Minute

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
	if err := s.auth.RecentLogin(bearer(r), time.Now(), withdrawMaxLoginAge); err != nil {
		Error(w, http.StatusUnauthorized, "reauth_required", "退会する前にもう一度ログインしてください")
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

	var deleteAuth func() error
	if s.authAdmin.Enabled() {
		deleteAuth = func() error {
			return s.authAdmin.DeleteUser(r.Context(), uid)
		}
	} else {
		slog.Warn("supabase service role key is not set; auth user was not deleted", "user_id", uid.String())
	}

	authDeleted, err := finishWithdraw(func() error {
		return tx.Commit(r.Context())
	}, deleteAuth)
	if err != nil && !errors.Is(err, errAuthDelete) {
		slog.Error("account delete commit failed", "err", err, "user_id", uid.String())
		MapDBError(w, err)
		return
	}
	if errors.Is(err, errAuthDelete) {
		slog.Error("auth user remains after account purge", "err", err, "user_id", uid.String())
	}
	JSON(w, http.StatusOK, map[string]bool{"auth_deleted": authDeleted})
}

// finishWithdraw commits the purge before deleting the Auth user. A commit
// failure leaves the Auth user in place. An Auth failure after commit is
// returned as errAuthDelete so the caller can retry that deletion.
func finishWithdraw(commit func() error, deleteAuth func() error) (bool, error) {
	if err := commit(); err != nil {
		return false, err
	}
	if deleteAuth == nil {
		return false, nil
	}
	if err := deleteAuth(); err != nil {
		return false, fmt.Errorf("%w: %w", errAuthDelete, err)
	}
	return true, nil
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
