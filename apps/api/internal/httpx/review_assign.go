package httpx

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/jackc/pgx/v5"
)

// 評価が低いコーチを候補から外す基準。
const (
	lowRatingMinCount = 5
	lowRatingStars    = 3.0
)

// dayStart は日本時間の今日の0時。1日の上限本数はこの時刻から数える。
func dayStart(now time.Time) time.Time {
	local := now.In(jst.Zone)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, jst.Zone)
}

// estimateReviewDue は返却期限の見込み。いつ送っても、送った時刻から24時間後。
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

// assignReview makes the paid request visible and records the work order in the same transaction.
func (s *Server) assignReview(ctx context.Context, q *db.Queries, rr db.ReviewRequest) (db.ReviewRequest, error) {
	coachID := uuid.Nil
	if rr.RequestedCoachID != nil {
		coachID = *rr.RequestedCoachID
	} else {
		if !s.cfg.ReviewRandomAssign {
			return rr, nil
		}
		var err error
		coachID, err = q.PickCoachForRequest(ctx, db.PickCoachForRequestParams{
			Tier: rr.Tier, Event: rr.Event, RequesterID: rr.RequesterID,
			MinRatings: lowRatingMinCount, LowStars: lowRatingStars,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return rr, nil
		}
		if err != nil {
			return rr, err
		}
	}
	assigned, err := q.AssignReviewRequest(ctx, db.AssignReviewRequestParams{ID: rr.ID, CoachID: coachID})
	if errors.Is(err, pgx.ErrNoRows) {
		return rr, nil
	}
	if err != nil {
		return rr, err
	}
	name, err := coachDisplayName(ctx, q, coachID)
	if err != nil {
		return rr, err
	}
	terms, err := s.reviewWorkOrderTerms(ctx, q, assigned, name, rr.CreatedAt)
	if err != nil {
		return rr, err
	}
	if err = saveWorkOrder(ctx, q, "review", &rr.ID, nil, coachID, rr.ID, terms); err != nil {
		return rr, err
	}
	if err = q.TouchCoachOffered(ctx, coachID); err != nil {
		return rr, err
	}
	err = s.recordReviewNotice(ctx, q, coachID, rr.ID, "review_offer", push.Message{
		Title: "動画添削の依頼が届きました",
		URL:   "/coach/requests/" + rr.ID.String(),
	}, assigned.DueAt, "review_assigned:"+rr.ID.String())
	return assigned, err
}

func (s *Server) recordReviewNotice(ctx context.Context, q *db.Queries, to, id uuid.UUID, kind string, msg push.Message, expires time.Time, key string) error {
	if err := insertReviewNotificationWithURL(ctx, q, to, kind, id, &msg.URL); err != nil {
		return err
	}
	if !s.push.Configured() {
		return nil
	}
	category := kindCategory(kind)
	ok, err := notificationWanted(ctx, q, to, category, false)
	if err != nil || !ok {
		return err
	}
	return q.EnqueuePushOutbox(ctx, db.EnqueuePushOutboxParams{
		ID: uuid.New(), ProfileID: to, Title: msg.Title, Body: msg.Body, Url: msg.URL,
		Tag: key, Category: category, TtlSeconds: int32(max(1, time.Until(expires).Seconds())),
		DedupKey: key, ExpiresAt: expires,
	})
}

// offerReview retries assignment for paid work still in the queue.
func (s *Server) offerReview(ctx context.Context, id uuid.UUID) (bool, error) {
	var assigned bool
	err := s.tx(ctx, func(q *db.Queries) error {
		if err := q.LockReviewAssignment(ctx); err != nil {
			return err
		}
		rr, err := q.LockQueuedReviewRequest(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		rr, err = s.assignReview(ctx, q, rr)
		assigned = err == nil && rr.Status == "in_review"
		return err
	})
	return assigned, err
}

func (s *Server) remindReviews(ctx context.Context) error {
	return s.tx(ctx, func(q *db.Queries) error {
		for _, hours := range []int32{12, 4} {
			requests, err := q.ClaimReviewDueReminders(ctx, hours)
			if err != nil {
				return err
			}
			for _, rr := range requests {
				msg := push.Message{
					Title: fmt.Sprintf("フォーム添削の返却期限まであと%d時間", hours),
					Body:  reviewDeadlineText(rr.DueAt) + "までに添削を返却してください。",
					URL:   "/coach/requests/" + rr.ID.String(),
				}
				key := fmt.Sprintf("review_due:%s:%d:%d", rr.ID, hours, rr.DueAt.UnixMicro())
				if err := s.recordReviewNotice(ctx, q, *rr.CoachID, rr.ID, "review_offer_reminder", msg, rr.DueAt, key); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// dispatchQueuedReviews は、待機中の依頼を古い順に割り当てる。
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
	// 決済完了から6か月を過ぎて失効させたチケットの枚数（本体 #569）。
	TicketsExpired int `json:"tickets_expired"`
	// 失効の30日前・7日前に送ったチケットの期限の知らせ（本体 #569）。
	TicketExpiryReminders int `json:"ticket_expiry_reminders"`
	Expired               int `json:"expired"`
	// 再提出期限までに再提出がなく終わった差し戻し。
	Unresubmitted int `json:"unresubmitted"`
	Requeued      int `json:"requeued"`
	Offered       int `json:"offered"`
	HelpfulAsk    int `json:"helpful_ask"`
	// パーソナル指導: 返事のないまま期限が過ぎた申し込みと、期間が終わったもの。
	ContractsExpired       int `json:"contracts_expired"`
	SupportConsultsExpired int `json:"support_consults_expired"`
	ContractsEnded         int `json:"contracts_ended"`
	// 支払い: 自動で返金したもの（コーチが72時間書かなかった、始まらなかった）。
	PaymentsRefunded int `json:"payments_refunded"`
	// 添削の支払い: 目安を過ぎたなどで取り消し・返金したもの。
	ReviewPaymentsReleased int `json:"review_payments_released"`
	// コーチの報酬として記録したもの（返却した添削、終わったパーソナル指導）。
	EarningsRecorded int `json:"earnings_recorded"`
	// 期間が終わって1年たち、やりとりを消した契約。
	MessagesPurged int `json:"messages_purged"`
	// 送り直した、保護者への購入の通知。
	GuardianNoticesSent int `json:"guardian_notices_sent"`
	// 待ちに入れていた Push のうち、送り直して届いたもの（本体 #421 F04）。
	PushResent int `json:"push_resent"`
}

// runCoachingTick は、返却期限の処理・リマインド・決済の再試行を行う。
// 返却から7日たった返答には「役に立ちましたか」を聞く。パーソナル指導の申し込みの期限切れと、期間の終わりも片付ける。
func (s *Server) runCoachingTick(ctx context.Context) (coachingTickResult, error) {
	var out coachingTickResult
	overdue, err := s.expireOverdueReviews(ctx)
	if err != nil {
		return out, err
	}
	out.Expired = len(overdue)
	for _, rr := range overdue {
		s.notifyReviewExpired(ctx, rr)
	}
	if out.Unresubmitted, err = s.expireUnresubmittedReviews(ctx); err != nil {
		return out, err
	}
	out.Offered = s.dispatchQueuedReviews(ctx)
	if err := s.remindReviews(ctx); err != nil {
		slog.Warn("review offer reminder", "err", err)
	}
	// 送れなかった Push を送り直す（本体 #421 F04）。期限切れの待ちは送らない。
	out.PushResent = s.sendPendingOutbox(ctx, pushOutboxBatch)
	if out.HelpfulAsk, err = s.askHelpful(ctx); err != nil {
		return out, err
	}
	if out.ContractsExpired, out.ContractsEnded, err = s.closeCoachingContracts(ctx); err != nil {
		return out, err
	}
	if out.SupportConsultsExpired, err = s.expireSupportConsults(ctx); err != nil {
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
	if out.TicketsExpired, err = s.expireTickets(ctx); err != nil {
		return out, err
	}
	if out.TicketExpiryReminders, err = s.remindTicketExpiry(ctx); err != nil {
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
	s.retryRefundCompletionNotices(ctx)
	if out.ConsultationsPurged, err = s.purgeConsultations(ctx); err != nil {
		return out, err
	}
	if out.MessagesPurged, err = s.purgeCoachingMessages(ctx); err != nil {
		return out, err
	}
	if out.GuardianNoticesSent, err = s.retryGuardianNotices(ctx); err != nil {
		return out, err
	}
	return out, nil
}

func (s *Server) notifyReviewExpired(ctx context.Context, rr db.ExpireOverdueReviewRequestsRow) {
	if err := insertReviewNotification(ctx, s.q, rr.RequesterID, "review_expired", rr.ID); err != nil {
		slog.Warn("review expired notification", "err", err)
	}
	// 期限切れ系の通知は1通に統合する（本体 #573）。取り消しの別通知は出さない。
	// 文面は表題1行だけにし、取り消し・返金・チケットの返還は依頼の画面で見せる（本体 #609）。
	s.pushQuietly(ctx, rr.RequesterID, notifyReview, push.Message{
		Title: "フォーム添削の依頼が期限を過ぎました",
		URL:   "/coaching/requests/" + rr.ID.String(),
		Tag:   "review-" + rr.ID.String(),
	}, 24*time.Hour)
	if rr.OldStatus == "in_review" && rr.CoachID != nil {
		s.notify(ctx, *rr.CoachID, notifyReview, push.Message{
			Title: "添削中の依頼が期限を過ぎました",
			Body:  "返却期限を過ぎたため依頼を終了し、新規受付をオフにしました",
			URL:   "/coach",
			Tag:   "review-" + rr.ID.String(),
		}, 24*time.Hour)
	}
	if rr.OldStatus != "in_review" && rr.RequestedCoachID != nil {
		s.notifyCoachNominationEnded(ctx, *rr.RequestedCoachID, rr.ID)
	}
}

func (s *Server) notifyCoachNominationEnded(ctx context.Context, coachID, requestID uuid.UUID) {
	if err := insertReviewNotification(ctx, s.q, coachID, "review_nomination_ended", requestID); err != nil {
		slog.Warn("coach nomination ended notification", "err", err)
	}
	s.pushQuietly(ctx, coachID, notifyReview, push.Message{
		Title: "指名されたフォーム添削の依頼が終了しました",
		URL:   "/coach",
		Tag:   "review-ended-" + requestID.String(),
	}, 24*time.Hour)
}

// notifyNominationEnded は、コーチを選んだ依頼が辞退・打診の時間切れで終わったら依頼した人に知らせる。
// 選んだコーチの依頼は別のコーチへ回さないので、ここで終わる。代金とチケットは tick が戻す。
func (s *Server) notifyNominationEnded(ctx context.Context, id uuid.UUID, notifyCoach bool) {
	rr, err := s.q.GetReviewRequest(ctx, id)
	if err != nil || rr.RequestedCoachID == nil || rr.Status != "expired" {
		return
	}
	if err := insertReviewNotification(ctx, s.q, rr.RequesterID, "review_expired", rr.ID); err != nil {
		slog.Warn("nomination ended notification", "err", err)
	}
	s.pushQuietly(ctx, rr.RequesterID, notifyReview, push.Message{
		Title: "選んだコーチが依頼を引き受けられませんでした",
		URL:   "/coaching/requests/" + rr.ID.String(),
		Tag:   "review-" + rr.ID.String(),
	}, 24*time.Hour)
	if notifyCoach {
		s.notifyCoachNominationEnded(ctx, *rr.RequestedCoachID, rr.ID)
	}
}

// coachingTick は Cloud Scheduler から1分ごとに呼ばれる。共有の秘密のヘッダーがないと受け付けない。
func (s *Server) coachingTick(w http.ResponseWriter, r *http.Request) {
	secret := s.cfg.CoachingTickSecret
	got := r.Header.Get("X-Coaching-Tick-Secret")
	if secret == "" || subtle.ConstantTimeCompare([]byte(got), []byte(secret)) != 1 {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	s.retryNotificationJobs(r.Context())
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
		"started_at":         rr.StartedAt,
		"due_at":             rr.DueAt,
		"created_at":         rr.CreatedAt,
		"requested_coach_id": rr.RequestedCoachID,
		"coach_reward_yen":   rr.BasePriceYen * reviewSharePercent / 100,
		"info_request":       infoRequestJSON(rr),
		"withdrawal":         withdrawalJSON(rr),
	}
	out["videos_purged"] = rr.VideosPurgedAt != nil
	if withVideo {
		out["video_url"] = reviewVideoURL(rr, s.cloud.CoachingPlaybackURL)
		out["thumbnail_url"] = reviewVideoURL(rr, s.cloud.CoachingThumbnailURL)
	}
	return out
}

// listCoachRequests は、コーチの対応中一覧。
func (s *Server) listCoachRequests(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireApprovedReviewCoach(w, r)
	if !ok {
		return
	}
	reviewing, err := s.q.ListCoachReviewing(r.Context(), p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	reviewOut := make([]map[string]any, 0, len(reviewing))
	for _, rr := range reviewing {
		reviewOut = append(reviewOut, s.coachRequestJSON(rr, coachSeesVideo(rr)))
	}
	JSON(w, http.StatusOK, map[string]any{"offers": []any{}, "reviewing": reviewOut})
}
