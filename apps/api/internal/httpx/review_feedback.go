package httpx

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/jackc/pgx/v5"
)

// 返却された添削への匿名の感想（本体 #911）。
// 感想は担当コーチにだけ届くフィードバックで、公開評価ではない。並び順・受付可否・報酬などには使わない。
// コーチには送信から7日たってから、本文だけを見せる。どの選手・どの依頼かが分かる情報は一切出さない。

const reviewFeedbackMaxRunes = 500

// 感想が見えるようになったときのコーチへの知らせ。本文・選手・依頼には触れない固定文にする。
const (
	reviewFeedbackNoticeTitle = "選手からの感想が届きました"
	reviewFeedbackNoticeBody  = "新しい感想が届いています。"
	reviewFeedbackNoticeURL   = "/coach/feedback"
)

type reviewFeedbackReq struct {
	Body *string `json:"body"`
}

// submitReviewFeedback は、返答が届いた自分の依頼に、匿名の感想を1回だけ送る。
func (s *Server) submitReviewFeedback(w http.ResponseWriter, r *http.Request) {
	rr, ok := s.requesterReview(w, r)
	if !ok {
		return
	}
	var req reviewFeedbackReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	body := ""
	if req.Body != nil {
		body = strings.TrimSpace(*req.Body)
	}
	if n := utf8.RuneCountInString(body); n == 0 || n > reviewFeedbackMaxRunes {
		JSON(w, http.StatusBadRequest, errorBody{Error: errDetail{
			Code:    "validation_error",
			Message: "入力内容を確認してください",
			Fields:  map[string]string{"body": "感想は1〜500文字で入力してください"},
		}})
		return
	}
	if rr.Status != "returned" && rr.Status != "clarifying" {
		Error(w, http.StatusConflict, "conflict", "返答が届いてから感想を送れます")
		return
	}
	// 届け先は返却したコーチ。選手には選ばせない。
	if rr.CoachID == nil {
		Error(w, http.StatusConflict, "conflict", "返答が届いてから感想を送れます")
		return
	}
	visibleAt, err := s.q.InsertReviewFeedback(r.Context(), db.InsertReviewFeedbackParams{
		ID:        uuid.New(),
		RequestID: rr.ID,
		CoachID:   *rr.CoachID,
		Body:      body,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "conflict", "この添削への感想はすでに送っています")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusCreated, map[string]any{"feedback_submitted": true, "visible_at": visibleAt})
}

// listCoachFeedback は、表示開始を過ぎた自分宛ての感想を、本文だけで返す。
// 依頼ID・選手・種目・日付は返さない（どの添削への感想かを分からなくするため）。
func (s *Server) listCoachFeedback(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireApprovedReviewCoach(w, r)
	if !ok {
		return
	}
	rows, err := s.q.ListVisibleCoachFeedbacks(r.Context(), p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	type item struct {
		ID   uuid.UUID `json:"id"`
		Body string    `json:"body"`
	}
	out := make([]item, 0, len(rows))
	for _, row := range rows {
		out = append(out, item{ID: row.ID, Body: row.Body})
	}
	JSON(w, http.StatusOK, map[string]any{"feedbacks": out})
}

// attachFeedbackSubmitted は、依頼詳細（選手本人向け）に、感想を送ったかどうかだけを足す。
func (s *Server) attachFeedbackSubmitted(ctx context.Context, out map[string]any, requestID uuid.UUID) error {
	submitted, err := s.q.ReviewFeedbackSubmitted(ctx, requestID)
	if err != nil {
		return err
	}
	out["feedback_submitted"] = submitted
	return nil
}

// notifyVisibleFeedbacks は、表示開始を過ぎた感想について、担当コーチに一度だけ知らせる。tick から呼ぶ。
// 送信時点では知らせない（届いた時刻から選手を特定されないようにするため）。
// 知らせは既存の message の知らせ（ベル・プッシュとも本人の「1回プラン」の設定に従う）。
func (s *Server) notifyVisibleFeedbacks(ctx context.Context) (int, error) {
	coaches, err := s.q.MarkReviewFeedbacksNotified(ctx)
	if err != nil {
		return 0, err
	}
	for _, coachID := range coaches {
		s.notify(ctx, coachID, notifyReview, push.Message{
			Title: reviewFeedbackNoticeTitle,
			Body:  reviewFeedbackNoticeBody,
			URL:   reviewFeedbackNoticeURL,
			Tag:   "review-feedback",
		}, 24*time.Hour)
	}
	return len(coaches), nil
}
