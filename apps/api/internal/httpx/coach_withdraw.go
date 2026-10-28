package httpx

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
)

const coachWithdrawConfirm = "コーチをやめる"

type withdrawCoachReq struct {
	Confirm string `json:"confirm"`
}

// withdrawCoach は、コーチだけをやめる（審査待ちの応募なら取り下げる）。アカウントは残り、一般の利用者として使い続けられる。
// コーチ一覧・プラン・空き状況から外れ、新しい依頼は来なくなる。もう一度応募すれば審査からやり直す。
// 受けている仕事が残っているあいだはやめさせない。残りの報酬は、最低額に届かなくても翌月にまとめて送る。
func (s *Server) withdrawCoach(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var req withdrawCoachReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if req.Confirm != coachWithdrawConfirm {
		FieldError(w, map[string]string{"confirm": "確認のため「コーチをやめる」と入力してください"})
		return
	}

	// 口座登録が済んだかを先に Stripe に聞いて記録する（この下でコーチの行をロックするので、その前に）。
	if c, err := s.q.GetCoachProfile(r.Context(), p.ID); err == nil {
		s.refreshPayoutsEnabled(r.Context(), c)
	}

	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)

	coach, err := q.LockCoachProfile(r.Context(), p.ID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && coach.Status != "pending" && coach.Status != "approved") {
		Error(w, http.StatusConflict, "conflict", "いまはコーチとして活動していません")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	blockers, err := q.CoachWithdrawBlockers(r.Context(), p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if blockers.OpenContract {
		Error(w, http.StatusConflict, "coaching_in_progress", "進行中のパーソナル指導があるあいだはコーチをやめられません。終わってからやめるか、お問い合わせからご連絡ください")
		return
	}
	if blockers.OpenReview {
		Error(w, http.StatusConflict, "review_in_progress", "添削中の依頼があるあいだはコーチをやめられません。返却してからやめてください")
		return
	}
	if blockers.OpenConsultation {
		Error(w, http.StatusConflict, "consultations_open", "対応中の追加相談・確認質問・返金があるあいだはコーチをやめられません。完了してからやめてください")
		return
	}

	// やめたあとは報酬の画面を開けないので、残りの報酬を受け取る口座がないままやめさせない。
	for {
		n, err := recordCoachEarningsWith(r.Context(), q)
		if err != nil {
			MapDBError(w, err)
			return
		}
		if n == 0 {
			break
		}
	}
	summary, err := q.GetCoachEarningsSummary(r.Context(), &p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if summary.UnpaidYen > 0 && !s.payoutsReady(coach) {
		Error(w, http.StatusConflict, "payouts_not_ready", "まだ送金していない報酬があります。コーチの画面の「報酬」から受け取り口座を登録してからやめてください")
		return
	}

	requeued, err := q.DeclineCoachOpenOffers(r.Context(), p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	oldPhoto, err := q.WithdrawCoach(r.Context(), p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	// 特商法の登録情報は消さずに残し、保持期間の起算点を書き留める。
	if err := q.EndCoachLegalIdentity(r.Context(), p.ID); err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}

	if oldPhoto != nil {
		s.deleteUnusedMedia(r.Context(), postMedia{Images: []string{*oldPhoto}})
	}
	// 断った扱いにした依頼は、ほかのコーチに打診し直す。
	for _, id := range requeued {
		if _, err := s.offerReview(r.Context(), id); err != nil {
			slog.Warn("review offer after coach withdraw", "err", err)
		}
	}
	c, err := s.q.GetCoachProfile(r.Context(), p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"application": s.coachProfileJSON(c)})
}
