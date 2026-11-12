package httpx

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/jackc/pgx/v5"
)

// 動画添削の「引受後のコーチ都合の辞退」（運営者決定 2026-11-03）。
//
//   - 使えるのは担当のコーチだけ。引き受けたあと、返却するまで（添削中、または引受後に差し戻して再提出を待っている間）。
//     引受前の打診は、これまでどおり見送り（/coach/offers/{id}/decline）で断る。
//   - 辞退した依頼はその場で終わる。支払いの取り消し・返金（カードは仮押さえの取り消し、PayPay は全額返金）と
//     チケットの返却は、既存の期限切れと同じく tick が行う。ここで別の返金の経路は作らない。
//   - 別のコーチには回さない（指名制）。ペナルティや手数料はない。
//   - 依頼の状態を変えるだけでメッセージではないので、メッセージ機能のフラグには入れない。
const maxWithdrawReasonChars = 200

// withdrawalJSON は、依頼に付ける辞退の記録。辞退していなければ nil。
func withdrawalJSON(rr db.ReviewRequest) map[string]any {
	if rr.WithdrawnAt == nil {
		return nil
	}
	return map[string]any{
		"withdrawn_at": rr.WithdrawnAt,
		"from_status":  rr.WithdrawnFromStatus,
		"reason":       rr.WithdrawReason,
		"reason_code":  rr.ReturnReason,
	}
}

type withdrawReq struct {
	Reason     *string `json:"reason"`
	ReasonCode string  `json:"reason_code"`
}

// withdrawReviewRequest は、担当のコーチが引き受けた依頼を返却する前に辞退する。
func (s *Server) withdrawReviewRequest(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireApprovedReviewCoach(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req withdrawReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if !oneOf(req.ReasonCode, []string{"video_unviewable", "wrong_event", "medical", "other"}) {
		FieldError(w, map[string]string{"reason_code": "返却理由を選んでください"})
		return
	}
	var reason *string
	if req.Reason != nil {
		if v := strings.TrimSpace(*req.Reason); v != "" {
			switch {
			case utf8.RuneCountInString(v) > maxWithdrawReasonChars:
				FieldError(w, map[string]string{"reason": "200文字以内で書いてください"})
				return
			case looksLikeContact(v):
				FieldError(w, map[string]string{"reason": "URL・メールアドレス・電話番号・SNS の ID など、連絡先らしきものは書けません"})
				return
			}
			reason = &v
		}
	}
	current, err := s.q.GetReviewRequest(r.Context(), id)
	if err == nil && (current.CoachID == nil || *current.CoachID != p.ID) {
		err = pgx.ErrNoRows
	}
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusNotFound, "not_found", "依頼が見つかりません")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	rr, err := s.q.WithdrawReviewRequest(r.Context(), db.WithdrawReviewRequestParams{Reason: reason, ReturnReason: req.ReasonCode, ID: id, CoachID: p.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		if current.WithdrawnAt != nil {
			Error(w, http.StatusConflict, "already_withdrawn", "この依頼はすでに返却済みです")
			return
		}
		Error(w, http.StatusConflict, "conflict", "この依頼はいま返却できません")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	s.notifyReview(r.Context(), rr.RequesterID, rr.ID, "review_withdrawn", reviewWithdrawalPush(rr.ID, s.stripe.Configured()))
	out := s.coachRequestJSON(rr, false)
	JSON(w, http.StatusOK, out)
}

func reviewWithdrawalPush(requestID uuid.UUID, paymentConfigured bool) push.Message {
	body := "担当のコーチの都合で、この依頼は終了しました"
	if paymentConfigured {
		body += "。お支払いは取り消します（PayPay は返金します）"
	}
	return push.Message{
		Title: "コーチが依頼を返却しました",
		Body:  body,
		URL:   "/coaching/requests/" + requestID.String(),
		Tag:   "review-" + requestID.String(),
	}
}
