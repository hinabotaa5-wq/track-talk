package httpx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
)

var errAuthDelete = errors.New("auth delete")

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

	if err := s.q.WithTx(tx).LockReviewRequester(r.Context(), uid); err != nil {
		MapDBError(w, err)
		return
	}
	consultationsOpen, err := s.q.WithTx(tx).HasOpenConsultationAccount(r.Context(), &uid)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if consultationsOpen {
		Error(w, 409, "consultations_open", "対応中の追加相談・確認質問・返金があります。完了後に退会できます")
		return
	}
	ticketsOpen, err := s.q.WithTx(tx).HasOpenTicketAccount(r.Context(), &uid)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if ticketsOpen {
		Error(w, http.StatusConflict, "tickets_remaining", "未使用・予約中のチケットや処理中の購入・返金があります。利用を終えるか、お問い合わせから返金をご相談ください")
		return
	}
	// 契約は退会でまとめて消えるので、進行中の指導や未解決の通報（運営が読む証拠）を消させない。
	blockers, err := s.q.WithTx(tx).WithdrawBlockers(r.Context(), uid)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if blockers.OpenContract {
		Error(w, http.StatusConflict, "coaching_in_progress", "進行中のパーソナル指導があるあいだは退会できません。終わってから退会するか、お問い合わせからご連絡ください")
		return
	}
	if blockers.OpenReviewPayment {
		Error(w, http.StatusConflict, "review_in_progress", "支払いが確定していないフォーム添削の依頼があるあいだは退会できません。返却されてから退会するか、お問い合わせからご連絡ください")
		return
	}
	if blockers.OpenReport {
		Error(w, http.StatusConflict, "report_in_progress", "パーソナル指導のやりとりへの通報を運営が確認しているあいだは退会できません。お問い合わせからご連絡ください")
		return
	}

	// 依頼や契約は退会で消えるので、tick がまだ記録していないコーチの報酬を先に記録する。
	// 記録は一度に100件までなので、残りがなくなるまでくり返す(返却・終了したものは退会を止めないため)。
	for {
		n, err := recordCoachEarningsWith(r.Context(), s.q.WithTx(tx))
		if err != nil {
			MapDBError(w, err)
			return
		}
		if n == 0 {
			break
		}
	}

	// 消える行が使っている写真・動画は、コミットしたあとで Cloudinary からも消す。
	mediaRows, err := s.q.WithTx(tx).ListAccountMedia(r.Context(), uid)
	if err != nil {
		MapDBError(w, err)
		return
	}
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
	var media postMedia
	var coachingVideos []string
	for _, m := range mediaRows {
		switch m.Resource {
		case "image":
			media.Images = append(media.Images, m.PublicID)
		case "video":
			media.Videos = append(media.Videos, m.PublicID)
		case "coaching":
			coachingVideos = append(coachingVideos, m.PublicID)
		}
	}
	s.purgeMedia(media)
	s.purgeCoachingVideos(coachingVideos)
	if errors.Is(err, errAuthDelete) {
		slog.Error("auth user remains after account purge", "err", err, "user_id", uid.String())
	}
	JSON(w, http.StatusOK, map[string]bool{"auth_deleted": authDeleted})
}

// finishWithdraw は、Auth のユーザーを消す前に、データの削除をコミットする。コミットに失敗したら Auth のユーザーは残す。
// コミットのあとで Auth の削除に失敗したら errAuthDelete を返し、呼び出し側がその削除だけをやり直せるようにする。
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
		q.RememberDeletedAccount,
		q.DeleteProfile,
	}
	for _, step := range steps {
		if err := step(ctx, userID); err != nil {
			return err
		}
	}
	return nil
}
