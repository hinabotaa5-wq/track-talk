package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/cloudinary"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// 添削の返答・補足・評価。
// コーチは良い点1つ、直す点1〜2つ、ドリル1つ（各300字）と、任意で見本動画を返す。
// 利用者は「ここがわからなかった」を1回だけ押せ、コーチは補足を1回だけ返す。★の評価と、7日後の「役に立ちましたか」もここ。

const maxResponseChars = 300

// 「ここがわからなかった」の理由。並びは画面に出す順。
var clarifyReasons = []string{"term", "drill", "fix", "other"}

var helpfulAnswers = []string{"yes", "some", "no"}

func oneOf(v string, options []string) bool {
	for _, o := range options {
		if o == v {
			return true
		}
	}
	return false
}

// responseTextError は、返答の1項目の文字数と連絡先らしき文字列を確かめる。問題がなければ空。
func responseTextError(label, value string) string {
	if value == "" {
		return label + "を入力してください"
	}
	if utf8.RuneCountInString(value) > maxResponseChars {
		return label + "は300文字以内で入力してください"
	}
	if looksLikeContact(value) {
		return "URL・メールアドレス・電話番号・SNS の ID など、連絡先らしきものは書けません"
	}
	return ""
}

// reviewResponseJSON は、返答を見せる形。運営が隠した返答は中身を出さない。
func (s *Server) reviewResponseJSON(rs db.ReviewResponse) map[string]any {
	if rs.HiddenAt != nil {
		return map[string]any{"hidden": true, "created_at": rs.CreatedAt}
	}
	out := map[string]any{
		"hidden":               false,
		"good_point":           rs.GoodPoint,
		"fix_points":           rs.FixPoints,
		"drill":                rs.Drill,
		"created_at":           rs.CreatedAt,
		"clarify_reason":       rs.ClarifyReason,
		"clarify_requested_at": rs.ClarifyRequestedAt,
		"clarification":        rs.Clarification,
		"clarified_at":         rs.ClarifiedAt,
		"helpful_asked":        rs.HelpfulAskedAt != nil,
		"demo_video_url":       nil,
	}
	if rs.DemoVideoPublicID != nil {
		out["demo_video_url"] = s.cloud.CoachingPlaybackURL(*rs.DemoVideoPublicID)
	}
	return out
}

// withResponse は、依頼の JSON に返答と評価を足す。
func (s *Server) withResponse(ctx context.Context, out map[string]any, requestID uuid.UUID) error {
	out["response"] = nil
	out["rating"] = nil
	rs, err := s.q.GetReviewResponse(ctx, requestID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	out["response"] = s.reviewResponseJSON(rs)
	rating, err := s.q.GetReviewRating(ctx, requestID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	out["rating"] = map[string]any{"stars": rating.Stars, "helpful": rating.Helpful}
	return nil
}

// requesterReview は、依頼した本人の依頼を返す。他の人には無いものとして返す。
func (s *Server) requesterReview(w http.ResponseWriter, r *http.Request) (db.ReviewRequest, bool) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return db.ReviewRequest{}, false
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return db.ReviewRequest{}, false
	}
	rr, err := s.q.GetReviewRequest(r.Context(), id)
	if err == nil && rr.RequesterID != p.ID {
		err = pgx.ErrNoRows
	}
	if err != nil {
		MapDBError(w, err)
		return db.ReviewRequest{}, false
	}
	return rr, true
}

func (s *Server) notifyReview(ctx context.Context, to, requestID uuid.UUID, kind string, msg push.Message) {
	if err := s.q.InsertReviewNotification(ctx, db.InsertReviewNotificationParams{ID: uuid.New(), RecipientID: to, Kind: kind, ReviewRequestID: &requestID}); err != nil {
		slog.Warn("review notification", "kind", kind, "err", err)
	}
	s.pushQuietly(ctx, to, msg, 24*time.Hour)
}

// getCoachReviewRequest は、引き受けたコーチに依頼と自分の返答を見せる。
func (s *Server) getCoachReviewRequest(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	rr, err := s.q.GetCoachReviewRequest(r.Context(), db.GetCoachReviewRequestParams{ID: id, CoachID: p.ID})
	if err != nil {
		MapDBError(w, err)
		return
	}
	out := s.coachRequestJSON(rr, true)
	if err := s.withResponse(r.Context(), out, rr.ID); err != nil {
		MapDBError(w, err)
		return
	}
	delete(out, "rating")
	JSON(w, http.StatusOK, out)
}

type reviewResponseReq struct {
	GoodPoint         string   `json:"good_point"`
	FixPoints         []string `json:"fix_points"`
	Drill             string   `json:"drill"`
	DemoVideoPublicID string   `json:"demo_video_public_id"`
}

// submitReviewResponse は、引き受けたコーチが返答を送る。返却目安を過ぎたら送れない。
func (s *Server) submitReviewResponse(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req reviewResponseReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	good := strings.TrimSpace(req.GoodPoint)
	drill := strings.TrimSpace(req.Drill)
	fixes := []string{}
	for _, f := range req.FixPoints {
		if f = strings.TrimSpace(f); f != "" {
			fixes = append(fixes, f)
		}
	}
	fields := map[string]string{}
	if e := responseTextError("良い点", good); e != "" {
		fields["good_point"] = e
	}
	switch {
	case len(fixes) == 0:
		fields["fix_points"] = "直す点を入力してください"
	case len(fixes) > 2:
		fields["fix_points"] = "直す点は2つまでです"
	default:
		for _, f := range fixes {
			if e := responseTextError("直す点", f); e != "" {
				fields["fix_points"] = e
				break
			}
		}
	}
	if e := responseTextError("ドリル", drill); e != "" {
		fields["drill"] = e
	}
	var demo *string
	if req.DemoVideoPublicID != "" {
		if !cloudinary.ValidCoachingPublicID(req.DemoVideoPublicID) {
			fields["demo_video_public_id"] = "見本動画を選び直してください"
		} else {
			demo = &req.DemoVideoPublicID
		}
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	if demo != nil {
		if err := s.cloud.CoachingVideoReady(r.Context(), *demo); err != nil {
			writeAssetReadyError(w, err, "見本動画")
			return
		}
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	rr, err := q.MarkReviewReturned(r.Context(), db.MarkReviewReturnedParams{ID: id, CoachID: &p.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "conflict", "この依頼にはもう返答できません")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	rs, err := q.CreateReviewResponse(r.Context(), db.CreateReviewResponseParams{
		RequestID:         rr.ID,
		CoachID:           &p.ID,
		GoodPoint:         good,
		FixPoints:         fixes,
		Drill:             drill,
		DemoVideoPublicID: demo,
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		Error(w, http.StatusConflict, "conflict", "この見本動画はもう使っています")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	// 返却したので、仮押さえしていた代金を確定する。
	s.captureReturnedReview(r.Context(), rr.ID)
	s.notifyReview(r.Context(), rr.RequesterID, rr.ID, "review_returned", push.Message{
		Title: "フォーム添削の返答が届きました",
		Body:  "コーチからのアドバイスを見てみましょう",
		URL:   "/coaching/requests/" + rr.ID.String(),
		Tag:   "review-" + rr.ID.String(),
	})
	out := s.coachRequestJSON(rr, true)
	out["response"] = s.reviewResponseJSON(rs)
	JSON(w, http.StatusOK, out)
}

type clarifyReq struct {
	Reason string `json:"reason"`
}

// clarifyReview は、利用者が「ここがわからなかった」を押す。1つの返答に1回だけ。
func (s *Server) clarifyReview(w http.ResponseWriter, r *http.Request) {
	rr, ok := s.requesterReview(w, r)
	if !ok {
		return
	}
	var req clarifyReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if !oneOf(req.Reason, clarifyReasons) {
		FieldError(w, map[string]string{"reason": "わからなかったところを選んでください"})
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	updated, err := q.MarkReviewClarifying(r.Context(), db.MarkReviewClarifyingParams{ID: rr.ID, RequesterID: rr.RequesterID})
	if err == nil {
		_, err = q.RequestReviewClarification(r.Context(), db.RequestReviewClarificationParams{Reason: req.Reason, RequestID: rr.ID})
	}
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "conflict", "補足はもう頼めません")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	if updated.CoachID != nil {
		s.notifyReview(r.Context(), *updated.CoachID, rr.ID, "review_clarify", push.Message{
			Title: "添削の補足を頼まれました",
			Body:  "わからなかったところに、補足を1回返してください",
			URL:   "/coach/requests/" + rr.ID.String(),
			Tag:   "review-" + rr.ID.String(),
		})
	}
	s.writeRequesterReview(w, r, updated)
}

type clarificationReq struct {
	Clarification string `json:"clarification"`
}

// submitClarification は、コーチが補足を1回だけ返す。
func (s *Server) submitClarification(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req clarificationReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	text := strings.TrimSpace(req.Clarification)
	if e := responseTextError("補足", text); e != "" {
		FieldError(w, map[string]string{"clarification": e})
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	rr, err := q.MarkReviewClarified(r.Context(), db.MarkReviewClarifiedParams{ID: id, CoachID: p.ID})
	var rs db.ReviewResponse
	if err == nil {
		rs, err = q.SaveReviewClarification(r.Context(), db.SaveReviewClarificationParams{Clarification: text, RequestID: id})
	}
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "conflict", "この依頼には補足を返せません")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	s.notifyReview(r.Context(), rr.RequesterID, rr.ID, "review_clarified", push.Message{
		Title: "添削の補足が届きました",
		Body:  "わからなかったところへの補足を見てみましょう",
		URL:   "/coaching/requests/" + rr.ID.String(),
		Tag:   "review-" + rr.ID.String(),
	})
	out := s.coachRequestJSON(rr, true)
	out["response"] = s.reviewResponseJSON(rs)
	JSON(w, http.StatusOK, out)
}

type reviewRatingReq struct {
	Stars   *int    `json:"stars"`
	Helpful *string `json:"helpful"`
}

// rateReview は、返答に★をつける。何度でもつけ直せる。
func (s *Server) rateReview(w http.ResponseWriter, r *http.Request) {
	rr, ok := s.requesterReview(w, r)
	if !ok {
		return
	}
	var req reviewRatingReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if req.Stars == nil || *req.Stars < 1 || *req.Stars > 5 {
		FieldError(w, map[string]string{"stars": "★を1〜5で選んでください"})
		return
	}
	if !s.reviewAnswered(w, rr) {
		return
	}
	if _, err := s.q.SaveReviewStars(r.Context(), db.SaveReviewStarsParams{RequestID: rr.ID, Stars: int16(*req.Stars)}); err != nil {
		MapDBError(w, err)
		return
	}
	s.writeRequesterReview(w, r, rr)
}

// answerHelpful は、7日後の「アドバイスは役に立ちましたか」に答える。
func (s *Server) answerHelpful(w http.ResponseWriter, r *http.Request) {
	rr, ok := s.requesterReview(w, r)
	if !ok {
		return
	}
	var req reviewRatingReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if req.Helpful == nil || !oneOf(*req.Helpful, helpfulAnswers) {
		FieldError(w, map[string]string{"helpful": "役に立ったかどうかを選んでください"})
		return
	}
	if !s.reviewAnswered(w, rr) {
		return
	}
	if _, err := s.q.SaveReviewHelpful(r.Context(), db.SaveReviewHelpfulParams{RequestID: rr.ID, Helpful: *req.Helpful}); err != nil {
		MapDBError(w, err)
		return
	}
	s.writeRequesterReview(w, r, rr)
}

// reviewAnswered は、返答が届いている依頼かを確かめる。まだなら 409 を書いて false。
func (s *Server) reviewAnswered(w http.ResponseWriter, rr db.ReviewRequest) bool {
	if rr.Status != "returned" && rr.Status != "clarifying" {
		Error(w, http.StatusConflict, "conflict", "返答が届いてから評価できます")
		return false
	}
	return true
}

func (s *Server) writeRequesterReview(w http.ResponseWriter, r *http.Request, rr db.ReviewRequest) {
	out := s.reviewRequestJSON(rr)
	if err := s.withResponse(r.Context(), out, rr.ID); err != nil {
		MapDBError(w, err)
		return
	}
	coach, err := s.reviewCoachJSON(r.Context(), out, rr)
	if err != nil {
		MapDBError(w, err)
		return
	}
	out["coach"] = coach
	if err := s.attachReviewPayment(r.Context(), out, rr.ID); err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, out)
}

// reviewCoachJSON は、返答と一緒に利用者へ見せるコーチのプロフィール。
// 返答が届いてから見せ、運営が返答を隠したときやコーチが退会したときは出さない。
func (s *Server) reviewCoachJSON(ctx context.Context, out map[string]any, rr db.ReviewRequest) (map[string]any, error) {
	resp, _ := out["response"].(map[string]any)
	if resp == nil || resp["hidden"] == true || rr.CoachID == nil {
		return nil, nil
	}
	c, err := s.q.GetReviewCoach(ctx, *rr.CoachID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"nickname":     c.Nickname,
		"photo_url":    s.coachPhotoURL(c.PhotoPublicID),
		"tier":         c.Tier,
		"events":       c.Events,
		"best_record":  c.BestRecord,
		"achievements": c.Achievements,
		"years":        c.Years,
	}, nil
}

// askHelpful は、返却から7日たった返答の利用者に「役に立ちましたか」を聞く。tick から呼ぶ。
func (s *Server) askHelpful(ctx context.Context) (int, error) {
	rows, err := s.q.MarkHelpfulAsksDue(ctx)
	if err != nil {
		return 0, err
	}
	for _, row := range rows {
		s.notifyReview(ctx, row.RequesterID, row.ID, "review_helpful_ask", push.Message{
			Title: "アドバイスは役に立ちましたか",
			Body:  "1週間前のフォーム添削について教えてください",
			URL:   "/coaching/requests/" + row.ID.String(),
			Tag:   "review-" + row.ID.String(),
		})
	}
	return len(rows), nil
}

func (s *Server) hideReviewResponse(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	n, err := s.q.HideReviewResponse(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if n == 0 {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// listCoachProbation は、各コーチの最初の20本の返答（試用期間）を運営に見せる。
func (s *Server) listCoachProbation(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	rows, err := s.q.ListCoachProbation(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	if rows == nil {
		rows = []db.ListCoachProbationRow{}
	}
	JSON(w, http.StatusOK, map[string]any{"responses": rows})
}
