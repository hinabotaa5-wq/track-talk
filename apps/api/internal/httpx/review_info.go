package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/hinata/track-talk/apps/api/internal/cloudinary"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/jackc/pgx/v5"
)

// 動画添削の「内容確認のための差し戻し」（運営者決定 2026-11-03）。
//
// 返却後に依頼者が聞く「ここがわからなかった」（/clarify と /clarification）とは別の仕組み。
// こちらは返却の前に、担当のコーチが依頼の内容を確かめるためのもの。
//
//   - 使えるのは担当のコーチだけ。引受前（打診中）でも引受後（添削中）でもよく、返却期限までに1依頼1回だけ。
//   - 「どこが・何が分からないか」の具体的な記述が要る。種類（任意）も選べる。
//   - 差し戻し中は辞退でも終了でもない。支払いの取り消し・返金は起こさない。
//   - 依頼者は差し戻しから24時間（夜間も止めない）以内に、相談内容を直して再提出する（動画も差し替えられる）。
//     期限までに再提出がなければ依頼は終わり、既存の期限切れと同じく取り消し・返金する（別のコーチには回さない）。
//   - 再提出すると、返却期限は再提出から24時間に数え直す。引受前に差し戻していたら、引受期限90分（夜間は数えない）も
//     数え直す。引受後に差し戻していたら、引き受けたまま添削に戻る。
const (
	// 依頼者が再提出できる時間。差し戻しから数え、夜間も止めない。
	reviewResubmitWindow = 24 * time.Hour
	// 差し戻しの質問の長さ。「意味が分からない」だけでは差し戻せないよう、短すぎる質問は断る。
	minInfoRequestChars = 15
	maxInfoRequestChars = 1000
)

// 差し戻しの種類。並びは画面に出す順。
var infoReasons = []string{"focus", "scene", "goal", "mismatch", "other"}

// meaningfulRunes は、空白と記号を除いた文字数。「？？？？」のような水増しを数えない。
func meaningfulRunes(s string) int {
	n := 0
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			n++
		}
	}
	return n
}

// infoRequestError は、差し戻しの質問の中身を確かめる。問題がなければ空。
func infoRequestError(body string) string {
	if body == "" {
		return "どこが・何が分からないかを書いてください"
	}
	if utf8.RuneCountInString(body) > maxInfoRequestChars {
		return "1000文字以内で書いてください"
	}
	if meaningfulRunes(body) < minInfoRequestChars {
		return "「意味が分からない」だけでは差し戻せません。どこが・何が分からないかを具体的に書いてください（15文字以上）"
	}
	if looksLikeContact(body) {
		return "URL・メールアドレス・電話番号・SNS の ID など、連絡先らしきものは書けません"
	}
	return ""
}

// coachSeesVideo は、コーチに動画を見せてよいか。動画は引き受けたあとにだけ見せる。
// 引受前に差し戻した依頼は、再提出されて引き受けるまで見せない。
func coachSeesVideo(rr db.ReviewRequest) bool {
	if rr.Status == "offered" || rr.Status == "queued" || rr.Status == "awaiting_payment" {
		return false
	}
	return !(rr.Status == "needs_info" && rr.InfoFromStatus != nil && *rr.InfoFromStatus == "offered")
}

// infoRequestJSON は、依頼に付ける差し戻しの履歴。差し戻していなければ nil。
func infoRequestJSON(rr db.ReviewRequest) map[string]any {
	if rr.InfoRequestedAt == nil {
		return nil
	}
	return map[string]any{
		"body":              rr.InfoRequest,
		"reason":            rr.InfoReason,
		"requested_at":      rr.InfoRequestedAt,
		"from_status":       rr.InfoFromStatus,
		"resubmit_by":       rr.ResubmitBy,
		"original_question": rr.OriginalQuestion,
		"video_replaced":    rr.VideoReplaced,
		"resubmitted_at":    rr.ResubmittedAt,
	}
}

type infoRequestReq struct {
	Body   string  `json:"body"`
	Reason *string `json:"reason"`
}

// requestReviewInfo は、担当のコーチが内容確認のために依頼を1回だけ差し戻す。
func (s *Server) requestReviewInfo(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireApprovedReviewCoach(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req infoRequestReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	body := strings.TrimSpace(req.Body)
	fields := map[string]string{}
	if e := infoRequestError(body); e != "" {
		fields["body"] = e
	}
	var reason *string
	if req.Reason != nil && *req.Reason != "" {
		if !oneOf(*req.Reason, infoReasons) {
			fields["reason"] = "種類を選び直してください"
		}
		reason = req.Reason
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	now := time.Now()
	resubmitBy := now.Add(reviewResubmitWindow)
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	current, err := q.GetReviewRequest(r.Context(), id)
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
	if current.InfoRequestedAt != nil {
		Error(w, http.StatusConflict, "info_already_requested", "この依頼はもう差し戻しています。差し戻しは1回だけです")
		return
	}
	if current.Status == "offered" {
		// 引受前の差し戻しは、引受期限内の打診があるときだけ。差し戻し中は再提出期限まで打診を残しておく。
		offer, err := q.LockCoachOpenOfferForRequest(r.Context(), db.LockCoachOpenOfferForRequestParams{RequestID: id, CoachID: p.ID})
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && !now.Before(offer.ExpiresAt)) {
			Error(w, http.StatusConflict, "offer_expired", "引き受けの期限が過ぎました")
			return
		}
		if err != nil {
			MapDBError(w, err)
			return
		}
	}
	rr, err := q.RequestReviewInfo(r.Context(), db.RequestReviewInfoParams{Body: body, Reason: reason, ResubmitBy: resubmitBy, ID: id, CoachID: p.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "conflict", "この依頼はいま差し戻せません")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	if rr.InfoFromStatus != nil && *rr.InfoFromStatus == "offered" {
		n, err := q.SetReviewOfferExpiry(r.Context(), db.SetReviewOfferExpiryParams{ExpiresAt: resubmitBy, RequestID: id, CoachID: p.ID})
		if err == nil && n != 1 {
			err = pgx.ErrNoRows
		}
		if errors.Is(err, pgx.ErrNoRows) {
			Error(w, http.StatusConflict, "offer_expired", "引き受けの期限が過ぎました")
			return
		}
		if err != nil {
			MapDBError(w, err)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	s.notifyReview(r.Context(), rr.RequesterID, rr.ID, "review_info_request", push.Message{
		Title: "コーチから依頼内容の確認が届きました",
		Body:  offerDeadlineText(resubmitBy) + "までに相談内容を直して再提出してください。再提出がなければ依頼は終了し、お支払いは取り消します（PayPay は返金します）",
		URL:   "/coaching/requests/" + rr.ID.String(),
		Tag:   "review-" + rr.ID.String(),
	})
	out := s.coachRequestJSON(rr, coachSeesVideo(rr))
	JSON(w, http.StatusOK, out)
}

type resubmitReq struct {
	Question *string `json:"question"`
	// 差し替える動画。空なら今の動画のまま。
	VideoPublicID string `json:"video_public_id"`
}

// resubmitReviewRequest は、差し戻された依頼を依頼者が直して再提出する。
func (s *Server) resubmitReviewRequest(w http.ResponseWriter, r *http.Request) {
	rr, ok := s.requesterReview(w, r)
	if !ok {
		return
	}
	var req resubmitReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if rr.Status != "needs_info" {
		Error(w, http.StatusConflict, "conflict", "この依頼はいま再提出できません")
		return
	}
	now := time.Now()
	if rr.ResubmitBy == nil || !now.Before(*rr.ResubmitBy) {
		Error(w, http.StatusConflict, "resubmit_expired", "再提出の期限が過ぎました")
		return
	}
	question := ""
	if req.Question != nil {
		question = strings.TrimSpace(*req.Question)
	}
	video := strings.TrimSpace(req.VideoPublicID)
	replace := video != "" && video != rr.VideoPublicID
	fields := map[string]string{}
	switch {
	case question == "":
		fields["question"] = "相談内容を記入してください"
	case utf8.RuneCountInString(question) > 1000:
		fields["question"] = "1000文字以内で記入してください"
	case question == strings.TrimSpace(rr.Question) && !replace:
		fields["question"] = "コーチの質問に答えるように、相談内容を直してください"
	}
	if replace && !cloudinary.ValidCoachingPublicID(video) {
		fields["video_public_id"] = "動画を選び直してください"
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	if replace {
		if !s.uploadIssuedToMe(w, r, rr.RequesterID, video, "video_public_id", "動画") {
			return
		}
		if err := s.cloud.CoachingVideoReady(r.Context(), video); err != nil {
			writeAssetReadyError(w, err, "動画")
			return
		}
	} else {
		video = rr.VideoPublicID
	}
	due := reviewDueAt(now)
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	if replace {
		if err := lockUnusedCoachingVideo(r.Context(), q, &video, nil); err != nil {
			var rule *ticketRuleError
			if errors.As(err, &rule) {
				Error(w, http.StatusConflict, "conflict", rule.message)
			} else {
				MapDBError(w, err)
			}
			return
		}
	}
	updated, err := q.ResubmitReviewRequest(r.Context(), db.ResubmitReviewRequestParams{
		Question: question, VideoPublicID: video, DueAt: due, ID: rr.ID, RequesterID: rr.RequesterID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "resubmit_expired", "再提出の期限が過ぎました")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	// 引受前に差し戻していたら、引受期限90分（夜間は数えない）を新しく数える。返却期限は越えない。
	var offerExpires time.Time
	if updated.Status == "offered" {
		if updated.CoachID == nil {
			Error(w, http.StatusConflict, "conflict", "この依頼はいま再提出できません")
			return
		}
		offerExpires = offerExpiresAt(now, offerWindow)
		if offerExpires.After(due) {
			offerExpires = due
		}
		n, err := q.SetReviewOfferExpiry(r.Context(), db.SetReviewOfferExpiryParams{ExpiresAt: offerExpires, RequestID: rr.ID, CoachID: *updated.CoachID})
		if err == nil && n != 1 {
			err = pgx.ErrNoRows
		}
		if errors.Is(err, pgx.ErrNoRows) {
			Error(w, http.StatusConflict, "resubmit_expired", "再提出の期限が過ぎました")
			return
		}
		if err != nil {
			MapDBError(w, err)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	if replace {
		// 差し替える前の動画は、この依頼だけが使っていた。残しても見る人がいないので消す。
		s.purgeCoachingVideos([]string{rr.VideoPublicID})
	}
	if updated.CoachID != nil {
		s.notifyResubmitted(r.Context(), updated, offerExpires)
	}
	s.writeRequesterReview(w, r, updated)
}

// notifyResubmitted は、差し戻した依頼が再提出されたことをコーチに知らせる。
func (s *Server) notifyResubmitted(ctx context.Context, rr db.ReviewRequest, offerExpires time.Time) {
	if rr.Status == "offered" {
		s.sendReviewResubmissionNotice(ctx, rr, "review_resubmitted", push.Message{
			Title: "差し戻した依頼が再提出されました",
			Body:  offerDeadlineText(offerExpires) + "までに、引き受けるかどうか決めてください",
			URL:   "/coach",
			Tag:   "review-offer",
		}, time.Until(offerExpires))
		return
	}
	s.sendReviewResubmissionNotice(ctx, rr, "review_resubmitted_after_accept", push.Message{
		Title: "差し戻した依頼が再提出されました",
		Body:  "返却期限は" + offerDeadlineText(rr.DueAt) + "です",
		URL:   "/coach/requests/" + rr.ID.String(),
		Tag:   "review-" + rr.ID.String(),
	}, 24*time.Hour)
}

func (s *Server) sendReviewResubmissionNotice(ctx context.Context, rr db.ReviewRequest, kind string, msg push.Message, ttl time.Duration) {
	if err := insertReviewNotificationWithURL(ctx, s.q, *rr.CoachID, kind, rr.ID, &msg.URL); err != nil {
		slog.Warn("review resubmitted notification", "err", err)
	}
	s.pushQuietly(ctx, *rr.CoachID, kindCategory(kind), msg, ttl)
}

// expireUnresubmittedReviews は、再提出期限を過ぎた差し戻しを終える。支払いの取り消し・返金は tick の既存の処理が行う。
func (s *Server) expireUnresubmittedReviews(ctx context.Context) (int, error) {
	ended, err := s.q.ExpireUnresubmittedReviewRequests(ctx)
	if err != nil {
		return 0, err
	}
	for _, rr := range ended {
		s.notifyUnresubmitted(ctx, rr)
	}
	return len(ended), nil
}

func (s *Server) notifyUnresubmitted(ctx context.Context, rr db.ExpireUnresubmittedReviewRequestsRow) {
	if err := insertReviewNotification(ctx, s.q, rr.RequesterID, "review_expired", rr.ID); err != nil {
		slog.Warn("review unresubmitted notification", "err", err)
	}
	body := "再提出の期限までに再提出がなかったため、依頼は終了しました"
	if s.stripe.Configured() {
		body += "。お支払いは取り消します（PayPay は返金します）"
	}
	s.pushQuietly(ctx, rr.RequesterID, notifyReview, push.Message{
		Title: "フォーム添削の依頼が終了しました",
		Body:  body,
		URL:   "/coaching/requests/" + rr.ID.String(),
		Tag:   "review-" + rr.ID.String(),
	}, 24*time.Hour)
	if rr.CoachID != nil {
		s.notify(ctx, *rr.CoachID, notifyReview, push.Message{
			Title: "差し戻した依頼が終了しました",
			Body:  "再提出の期限までに再提出がなかったため、この依頼は終わりました",
			URL:   "/coach",
			Tag:   "review-" + rr.ID.String(),
		}, 24*time.Hour)
	}
}
