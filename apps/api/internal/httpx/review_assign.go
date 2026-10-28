package httpx

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/jackc/pgx/v5"
)

// 添削の依頼をコーチに回す（割り当てと打診）。
// 依頼が来たとき、見送られたとき、打診が時間切れになったときに、受付中のコーチを1人選んで打診する。
// 時間切れは Cloud Scheduler から1分ごとに呼ばれる tick が処理する（API は使われないと0台まで縮むため）。

const (
	// 打診してから引き受けるかどうか決めるまでの時間。
	offerWindow = 30 * time.Minute
	// 1人のコーチが同時に持てる依頼（打診中・添削中の合計）。
	maxActiveReviews = 2
	// 評価が低いコーチを順番から外す基準。★の評価が lowRatingMinCount 件以上たまり、
	// 平均が lowRatingStars 未満なら打診しない。
	lowRatingMinCount = 5
	lowRatingStars    = 3.0
)

// dayStart は日本時間の今日の0時。1日の上限本数はこの時刻から数える。
func dayStart(now time.Time) time.Time {
	local := now.In(jst.Zone)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, jst.Zone)
}

// estimateReviewDue は返却の目安。いつ送っても、送った翌日の12時（日本時間）までにする。
// 2つ目の戻り値は、いま受付中のコーチがいるかどうか。
func (s *Server) estimateReviewDue(ctx context.Context, requester uuid.UUID, tier, event string, now time.Time) (time.Time, bool, error) {
	coaches, err := s.q.CountOnDutyCoaches(ctx, db.CountOnDutyCoachesParams{
		Tier:        tier,
		Event:       event,
		RequesterID: requester,
		MinRatings:  lowRatingMinCount,
		LowStars:    lowRatingStars,
	})
	if err != nil {
		return time.Time{}, false, err
	}
	return reviewDueAt(now), coaches > 0, nil
}

// offerReview は、待ちの依頼を受付中のコーチ1人に打診する。打診する相手がいなければ待ちのまま。
func (s *Server) offerReview(ctx context.Context, requestID uuid.UUID) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)
	if err := q.LockReviewAssignment(ctx); err != nil {
		return false, err
	}
	rr, err := q.LockQueuedReviewRequest(ctx, requestID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	now := time.Now()
	coachID, err := q.PickCoachForRequest(ctx, db.PickCoachForRequestParams{
		Tier:             rr.Tier,
		RequestedCoachID: rr.RequestedCoachID,
		Event:            rr.Event,
		RequesterID:      rr.RequesterID,
		RequestID:        rr.ID,
		MaxActive:        maxActiveReviews,
		DayStart:         dayStart(now),
		MinRatings:       lowRatingMinCount,
		LowStars:         lowRatingStars,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	expires := now.Add(offerWindow)
	if expires.After(rr.DueAt) {
		expires = rr.DueAt
	}
	if _, err := q.CreateReviewOffer(ctx, db.CreateReviewOfferParams{ID: uuid.New(), RequestID: rr.ID, CoachID: coachID, ExpiresAt: expires}); err != nil {
		return false, err
	}
	if err := q.MarkReviewRequestOffered(ctx, db.MarkReviewRequestOfferedParams{ID: rr.ID, CoachID: &coachID}); err != nil {
		return false, err
	}
	if err := q.TouchCoachOffered(ctx, coachID); err != nil {
		return false, err
	}
	if err := q.InsertReviewNotification(ctx, db.InsertReviewNotificationParams{ID: uuid.New(), RecipientID: coachID, Kind: "review_offer", ReviewRequestID: &rr.ID}); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	s.pushQuietly(ctx, coachID, notifyImportant, push.Message{
		Title: "フォーム添削の依頼が届きました",
		Body:  "30分以内に、引き受けるかどうか決めてください",
		URL:   "/coach",
		Tag:   "review-offer",
	}, offerWindow)
	return true, nil
}

// dispatchQueuedReviews は、待ちの依頼を古い順に打診する。打診できなかった依頼は次の機会に回す。
func (s *Server) dispatchQueuedReviews(ctx context.Context) int {
	ids, err := s.q.ListQueuedReviewRequests(ctx)
	if err != nil {
		slog.Warn("review dispatch: list queued", "err", err)
		return 0
	}
	offered := 0
	for _, id := range ids {
		ok, err := s.offerReview(ctx, id)
		if err != nil {
			slog.Warn("review dispatch: offer", "request", id, "err", err)
			continue
		}
		if ok {
			offered++
		}
	}
	return offered
}

// pushQuietly は Web プッシュを送る。その種類のプッシュを本人がオフにしていたら送らない。
// 届かなくても画面で見られるので、失敗は記録だけする。
func (s *Server) pushQuietly(ctx context.Context, to uuid.UUID, category string, msg push.Message, ttl time.Duration) {
	if !s.push.Configured() {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if ok, err := notificationWanted(ctx, s.q, to, category, false); err != nil || !ok {
		if err != nil {
			slog.Warn("notification setting", "err", err)
		}
		return
	}
	if _, err := s.sendPush(ctx, to, msg, ttl); err != nil {
		slog.Warn("review push", "err", err)
	}
}

type coachingTickResult struct {
	SupportSettled         int `json:"support_settled"`
	InPersonSettled        int `json:"in_person_settled"`
	ConsultationsExpired   int `json:"consultations_expired"`
	ConsultationRefunds    int `json:"consultation_refunds"`
	ConsultationsPurged    int `json:"consultations_purged"`
	TicketsReleased        int `json:"tickets_released"`
	TicketRefundsProcessed int `json:"ticket_refunds_processed"`
	Expired                int `json:"expired"`
	Requeued               int `json:"requeued"`
	Offered                int `json:"offered"`
	HelpfulAsk             int `json:"helpful_ask"`
	// パーソナル指導: 返事のないまま期限が過ぎた申し込みと、期間が終わったもの。
	ContractsExpired int `json:"contracts_expired"`
	ContractsEnded   int `json:"contracts_ended"`
	// 支払い: 自動で返金したもの（コーチが72時間書かなかった、始まらなかった）。
	PaymentsRefunded int `json:"payments_refunded"`
	// 添削の支払い: 目安を過ぎたなどで取り消し・返金したもの。
	ReviewPaymentsReleased int `json:"review_payments_released"`
	// コーチの報酬として記録したもの（返却した添削、終わったパーソナル指導）。
	EarningsRecorded int `json:"earnings_recorded"`
	// 期間が終わって1年たち、やりとりを消した契約。
	MessagesPurged int `json:"messages_purged"`
}

// runCoachingTick は、返却目安を過ぎた依頼と、返事のないまま30分たった打診を片付け、待ちの依頼を打診する。
// 返却から7日たった返答には「役に立ちましたか」を聞く。パーソナル指導の申し込みの期限切れと、期間の終わりも片付ける。
func (s *Server) runCoachingTick(ctx context.Context) (coachingTickResult, error) {
	var out coachingTickResult
	overdue, err := s.q.ExpireOverdueReviewRequests(ctx)
	if err != nil {
		return out, err
	}
	out.Expired = len(overdue)
	for _, rr := range overdue {
		s.notifyReviewExpired(ctx, rr)
	}
	requeued, err := s.q.ExpireStaleReviewOffers(ctx)
	if err != nil {
		return out, err
	}
	out.Requeued = len(requeued)
	for _, id := range requeued {
		s.notifyNominationEnded(ctx, id)
	}
	out.Offered = s.dispatchQueuedReviews(ctx)
	if out.HelpfulAsk, err = s.askHelpful(ctx); err != nil {
		return out, err
	}
	if out.ContractsExpired, out.ContractsEnded, err = s.closeCoachingContracts(ctx); err != nil {
		return out, err
	}
	if out.PaymentsRefunded, err = s.refundCoachingPayments(ctx); err != nil {
		return out, err
	}
	if out.TicketsReleased, err = s.releaseExpiredTickets(ctx); err != nil {
		return out, err
	}
	if out.TicketRefundsProcessed, err = s.settleTicketRefunds(ctx); err != nil {
		return out, err
	}
	if out.ReviewPaymentsReleased, err = s.settleReviewPayments(ctx); err != nil {
		return out, err
	}
	if out.SupportSettled, err = s.settleSupportContracts(ctx); err != nil {
		return out, err
	}
	if out.InPersonSettled, err = s.settleInPersonContracts(ctx); err != nil {
		return out, err
	}
	if out.EarningsRecorded, err = s.recordCoachEarnings(ctx); err != nil {
		return out, err
	}
	if out.ConsultationsExpired, err = s.expireConsultations(ctx); err != nil {
		return out, err
	}
	if out.ConsultationRefunds, err = s.settleConsultationRefunds(ctx); err != nil {
		return out, err
	}
	if out.ConsultationsPurged, err = s.purgeConsultations(ctx); err != nil {
		return out, err
	}
	if out.MessagesPurged, err = s.purgeCoachingMessages(ctx); err != nil {
		return out, err
	}
	return out, nil
}

func (s *Server) notifyReviewExpired(ctx context.Context, rr db.ExpireOverdueReviewRequestsRow) {
	if err := insertReviewNotification(ctx, s.q, rr.RequesterID, "review_expired", rr.ID); err != nil {
		slog.Warn("review expired notification", "err", err)
	}
	body := "返却の目安までに引き受けるコーチが見つかりませんでした"
	if rr.OldStatus == "in_review" {
		body = "返却の目安までにコーチの返答が届きませんでした"
	}
	if s.stripe.Configured() {
		body += "。お支払いは取り消します（PayPay は返金します）"
	}
	s.pushQuietly(ctx, rr.RequesterID, notifyReview, push.Message{
		Title: "フォーム添削の依頼が期限を過ぎました",
		Body:  body,
		URL:   "/coaching/requests/" + rr.ID.String(),
		Tag:   "review-" + rr.ID.String(),
	}, 24*time.Hour)
	if rr.OldStatus == "in_review" && rr.CoachID != nil {
		s.notify(ctx, *rr.CoachID, notifyReview, push.Message{
			Title: "添削中の依頼が期限を過ぎました",
			Body:  "返却の目安を過ぎたため、この依頼は終わりました",
			URL:   "/coach",
			Tag:   "review-" + rr.ID.String(),
		}, 24*time.Hour)
	}
}

// notifyNominationEnded は、指名した依頼が辞退・打診の時間切れで終わったら依頼した人に知らせる。
// 指名の依頼は別のコーチへ回さないので、ここで終わる。代金とチケットは tick が戻す。
func (s *Server) notifyNominationEnded(ctx context.Context, id uuid.UUID) {
	rr, err := s.q.GetReviewRequest(ctx, id)
	if err != nil || rr.RequestedCoachID == nil || rr.Status != "expired" {
		return
	}
	if err := insertReviewNotification(ctx, s.q, rr.RequesterID, "review_expired", rr.ID); err != nil {
		slog.Warn("nomination ended notification", "err", err)
	}
	s.pushQuietly(ctx, rr.RequesterID, notifyReview, push.Message{
		Title: "指名したコーチが依頼を引き受けられませんでした",
		Body:  "別のコーチには回さず、この依頼は終わりました。代金・指名料とチケットはお戻しします",
		URL:   "/coaching/requests/" + rr.ID.String(),
		Tag:   "review-" + rr.ID.String(),
	}, 24*time.Hour)
}

// coachingTick は Cloud Scheduler から1分ごとに呼ばれる。共有の秘密のヘッダーがないと受け付けない。
func (s *Server) coachingTick(w http.ResponseWriter, r *http.Request) {
	secret := s.cfg.CoachingTickSecret
	got := r.Header.Get("X-Coaching-Tick-Secret")
	if secret == "" || subtle.ConstantTimeCompare([]byte(got), []byte(secret)) != 1 {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	out, err := s.runCoachingTick(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, out)
}

// reviewEstimate は、送る画面に出す返却の目安。
func (s *Server) reviewEstimate(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	tier := r.URL.Query().Get("tier")
	event := r.URL.Query().Get("event")
	if !validCoachTier(tier) || !validReviewEvent(event) {
		FieldError(w, map[string]string{"event": "枠と種目を選んでください"})
		return
	}
	due, onDuty, err := s.estimateReviewDue(r.Context(), p.ID, tier, event, time.Now())
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{
		"due_at": due, "coaches_on_duty": onDuty,
		// payments が false のあいだは支払いなしのデモ版。
		"payments": s.stripe.Configured(), "price_yen": reviewPriceYen,
	})
}

// coachRequestJSON は、コーチに見せる依頼。依頼した人がわかる情報は出さない。
// 動画は引き受けたあとにだけ見られる。
func (s *Server) coachRequestJSON(rr db.ReviewRequest, withVideo bool) map[string]any {
	out := map[string]any{
		"id":                 rr.ID,
		"tier":               rr.Tier,
		"event":              rr.Event,
		"focus":              rr.Focus,
		"question":           rr.Question,
		"status":             rr.Status,
		"due_at":             rr.DueAt,
		"created_at":         rr.CreatedAt,
		"requested_coach_id": rr.RequestedCoachID, "nomination_fee_yen": rr.NominationFeeYen,
		"coach_reward_yen": rr.BasePriceYen*70/100 + rr.NominationFeeYen*80/100,
	}
	if withVideo {
		out["video_url"] = s.cloud.CoachingPlaybackURL(rr.VideoPublicID)
		out["thumbnail_url"] = s.cloud.CoachingThumbnailURL(rr.VideoPublicID)
	}
	return out
}

// listCoachRequests は、コーチに届いている打診と、引き受けて添削中の依頼。
func (s *Server) listCoachRequests(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireApprovedReviewCoach(w, r)
	if !ok {
		return
	}
	offers, err := s.q.ListCoachOpenOffers(r.Context(), p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	reviewing, err := s.q.ListCoachReviewing(r.Context(), p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	name, err := coachDisplayName(r.Context(), s.q, p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	now := time.Now()
	offerOut := make([]map[string]any, 0, len(offers))
	for _, o := range offers {
		// 引き受ける前に、この案件の取引条件を見せる。委託した日は引き受けた時になる。
		terms, err := s.reviewWorkOrderTerms(r.Context(), s.q, o.ReviewRequest, name, now)
		if err != nil {
			MapDBError(w, err)
			return
		}
		offerOut = append(offerOut, map[string]any{
			"id":         o.OfferID,
			"expires_at": o.OfferExpiresAt,
			"request":    s.coachRequestJSON(o.ReviewRequest, false),
			"terms":      terms,
		})
	}
	reviewOut := make([]map[string]any, 0, len(reviewing))
	for _, rr := range reviewing {
		reviewOut = append(reviewOut, s.coachRequestJSON(rr, true))
	}
	JSON(w, http.StatusOK, map[string]any{"offers": offerOut, "reviewing": reviewOut})
}

func (s *Server) acceptReviewOffer(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireApprovedReviewCoach(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	offer, err := q.LockOpenReviewOffer(r.Context(), db.LockOpenReviewOfferParams{ID: id, CoachID: p.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusNotFound, "not_found", "この依頼はもう引き受けられません")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	if !time.Now().Before(offer.ExpiresAt) {
		Error(w, http.StatusConflict, "offer_expired", "引き受けの期限が過ぎました")
		return
	}
	rr, err := q.AcceptReviewRequest(r.Context(), db.AcceptReviewRequestParams{ID: offer.RequestID, CoachID: &p.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "offer_expired", "この依頼はもう引き受けられません")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	if err := q.AnswerReviewOffer(r.Context(), db.AnswerReviewOfferParams{ID: offer.ID, Result: "accepted"}); err != nil {
		MapDBError(w, err)
		return
	}
	// 引き受けた時点の取引条件を書き留める(フリーランス保護法3条の明示)。
	name, err := coachDisplayName(r.Context(), q, p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	terms, err := s.reviewWorkOrderTerms(r.Context(), q, rr, name, time.Now())
	if err != nil {
		MapDBError(w, err)
		return
	}
	if err := saveWorkOrder(r.Context(), q, "review", &rr.ID, nil, p.ID, terms); err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	out := s.coachRequestJSON(rr, true)
	if wo, err := s.q.GetReviewWorkOrder(r.Context(), db.GetReviewWorkOrderParams{ReviewRequestID: &rr.ID, CoachID: &p.ID}); err == nil {
		out["work_order"] = workOrderJSON(wo)
	}
	JSON(w, http.StatusOK, out)
}

func (s *Server) declineReviewOffer(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireApprovedReviewCoach(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	offer, err := q.LockOpenReviewOffer(r.Context(), db.LockOpenReviewOfferParams{ID: id, CoachID: p.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusNotFound, "not_found", "この打診はもう終わっています")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	if err := q.AnswerReviewOffer(r.Context(), db.AnswerReviewOfferParams{ID: offer.ID, Result: "declined"}); err != nil {
		MapDBError(w, err)
		return
	}
	if err := q.RequeueReviewRequest(r.Context(), offer.RequestID); err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	if _, err := s.offerReview(r.Context(), offer.RequestID); err != nil {
		slog.Warn("review offer after decline", "err", err)
	}
	s.notifyNominationEnded(r.Context(), offer.RequestID)
	w.WriteHeader(http.StatusNoContent)
}
