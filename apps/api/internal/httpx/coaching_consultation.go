package httpx

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/cloudinary"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/jackc/pgx/v5"
)

type consultationRoom struct {
	id       uuid.UUID
	review   *db.ReviewRequest
	contract *db.CoachingContract
	owner    uuid.UUID
	coach    *uuid.UUID
	minor    bool
}

func (room consultationRoom) path(coach bool) string {
	if room.contract != nil {
		return "/coaching/personal/" + room.id.String()
	}
	if coach {
		return "/coach/requests/" + room.id.String()
	}
	return "/coaching/requests/" + room.id.String()
}
func consultationParent(c db.CoachingConsultation) (uuid.UUID, bool) {
	if c.ReviewRequestID != nil {
		return *c.ReviewRequestID, true
	}
	if c.ContractID != nil {
		return *c.ContractID, false
	}
	return uuid.Nil, false
}
func (s *Server) consultationRoot(ctx context.Context, q *db.Queries, id uuid.UUID, review bool) (consultationRoom, error) {
	room := consultationRoom{id: id}
	if review {
		r, err := q.GetReviewRequest(ctx, id)
		if err != nil {
			return room, err
		}
		room.review = &r
		room.owner = r.RequesterID
		room.coach = r.CoachID
		p, err := q.GetProfileByID(ctx, r.RequesterID)
		if err != nil {
			return room, err
		}
		room.minor = p.BirthMonth != nil && isMinor(*p.BirthMonth, time.Now())
	} else {
		c, err := q.GetCoachingContract(ctx, id)
		if err != nil {
			return room, err
		}
		room.contract = &c
		room.owner = c.RequesterID
		room.coach = &c.CoachID
		room.minor = c.Minor
	}
	return room, nil
}

// reviewConsultationFunding は、フォーム添削の追加相談の払い方を決める。購入済みのチケットが残っていれば
// チケット1枚、なければ申込み時の都度払い（カード・PayPay）。チケットの新規販売は止めた。
// 18歳未満の都度払いは、保護者が払う画面が契約のページにしかないので、まだ受け付けない。
func reviewConsultationFunding(ctx context.Context, q *db.Queries, room consultationRoom) (string, error) {
	balance, err := q.TicketBalance(ctx, &room.owner)
	if err != nil {
		return "", err
	}
	if balance.Available > 0 {
		return "ticket", nil
	}
	if room.minor {
		return "", &ticketRuleError{"minor_cash_unavailable", "18歳未満の方の追加相談のお支払いは準備中です"}
	}
	return "cash", nil
}
func (s *Server) consultationRoomAccess(w http.ResponseWriter, r *http.Request) (consultationRoom, string, bool) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return consultationRoom{}, "", false
	}
	review := strings.Contains(r.URL.Path, "/requests/") || strings.Contains(r.URL.Path, "/reviews/")
	room, err := s.consultationRoot(r.Context(), s.q, id, review)
	if err != nil {
		MapDBError(w, err)
		return room, "", false
	}
	if chi.URLParam(r, "token") != "" {
		minor, ok := s.guardianMinor(w, r)
		if !ok {
			return room, "", false
		}
		if room.owner != minor || !room.minor {
			Error(w, 404, "not_found", "見つかりませんでした")
			return room, "", false
		}
		return room, "guardian", true
	}
	p, ok := s.requireProfile(w, r)
	if !ok {
		return room, "", false
	}
	if p.ID == room.owner {
		return room, "requester", true
	}
	if room.coach != nil && *room.coach == p.ID {
		if !s.requireStaffMFA(w, r) {
			return room, "", false
		}
		return room, "coach", true
	}
	Error(w, 404, "not_found", "見つかりませんでした")
	return room, "", false
}
func (s *Server) consultationParty(w http.ResponseWriter, r *http.Request) (db.CoachingConsultation, string, bool) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return db.CoachingConsultation{}, "", false
	}
	c, err := s.q.GetConsultation(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return c, "", false
	}
	if chi.URLParam(r, "token") != "" {
		owner, ok := s.guardianMinor(w, r)
		if !ok {
			return c, "", false
		}
		if c.RequesterID == nil || *c.RequesterID != owner || !c.Minor {
			Error(w, 404, "not_found", "見つかりませんでした")
			return c, "", false
		}
		return c, "guardian", true
	}
	p, ok := s.requireProfile(w, r)
	if !ok {
		return c, "", false
	}
	if c.RequesterID != nil && *c.RequesterID == p.ID {
		return c, "requester", true
	}
	if c.CoachID != nil && *c.CoachID == p.ID && c.Status != "draft" && c.Status != "awaiting_payment" {
		if !s.requireStaffMFA(w, r) {
			return c, "", false
		}
		return c, "coach", true
	}
	Error(w, 404, "not_found", "見つかりませんでした")
	return c, "", false
}

// 期間内の含有分は通常チャットで使う。追加の動画だけは含有回数を使い切った場合に購入できる。
func (s *Server) consultationOffer(ctx context.Context, q *db.Queries, room consultationRoom) (int32, int16, bool, error) {
	if room.coach == nil {
		return 0, 0, false, &ticketRuleError{"coach_unavailable", "担当コーチがいないため追加相談は購入できません"}
	}
	cp, err := q.GetCoachProfile(ctx, *room.coach)
	if err != nil {
		return 0, 0, false, err
	}
	if cp.Status != "approved" || !s.payoutsReady(cp) {
		return 0, 0, false, &ticketRuleError{"coach_unavailable", "このコーチは追加相談を受け付けていません"}
	}
	if room.review != nil {
		rr := room.review
		if rr.Status != "returned" {
			return 0, 0, false, &ticketRuleError{"consultation_unavailable", "添削の返却・補足回答を待ってから相談できます"}
		}
		if !cp.OnDuty {
			return 0, 0, false, &ticketRuleError{"coach_unavailable", "このコーチは現在受付を停止しています"}
		}
		rs, err := q.GetReviewResponse(ctx, rr.ID)
		if err != nil {
			return 0, 0, false, err
		}
		if rr.Status != "returned" || rs.HiddenAt != nil {
			return 0, 0, false, &ticketRuleError{"consultation_unavailable", "添削の返却・補足回答を待ってから相談できます"}
		}
		return ticketFaceYen, reviewSharePercent, false, nil
	}
	c := room.contract
	if c.MessagesPurgedAt != nil || c.Status == "refunded" || c.Status == "suspended" {
		return 0, 0, false, &ticketRuleError{"consultation_unavailable", "この契約では追加相談を購入できません"}
	}
	if c.AdditionalConsultationPriceYen == nil {
		return 0, 0, false, &ticketRuleError{"consultation_unavailable", "この契約には追加相談の料金が設定されていません"}
	}
	plan, err := q.GetCoachingPlan(ctx, db.GetCoachingPlanParams{ID: c.PlanID, CoachID: c.CoachID})
	if err != nil {
		return 0, 0, false, err
	}
	if !plan.Published {
		return 0, 0, false, &ticketRuleError{"coach_unavailable", "このコーチはこのプランの受付を停止しています"}
	}
	requiresVideo := false
	switch c.Status {
	case "ended":
	case "active":
		if c.ServiceType == nil || c.VideoReviews == nil || *c.VideoReviews == 0 {
			return 0, 0, false, &ticketRuleError{"included_in_plan", "期間内の相談は通常のチャットをご利用ください"}
		}
		used, err := q.CountContractVideoSubmissions(ctx, c.ID)
		if err != nil {
			return 0, 0, false, err
		}
		if used < int32(*c.VideoReviews) {
			return 0, 0, false, &ticketRuleError{"included_in_plan", "プランに含まれる動画添削が残っています。通常のチャットをご利用ください"}
		}
		requiresVideo = true
	default:
		return 0, 0, false, &ticketRuleError{"consultation_unavailable", "指導開始後または終了後に相談できます"}
	}
	return *c.AdditionalConsultationPriceYen, 100 - personalFeePercent, requiresVideo, nil
}
func (s *Server) consultationJSON(c db.CoachingConsultation) map[string]any {
	out := map[string]any{"id": c.ID, "request_key": c.RequestKey, "status": c.Status, "funding": c.Funding, "price_yen": c.PriceYen, "payment_state": c.PaymentState,
		"minor": c.Minor, "question": c.Question, "answer": c.Answer, "video_public_id": c.VideoPublicID, "video_url": nil, "answer_video_url": nil,
		"clarify_body": c.ClarifyBody, "clarification": c.Clarification, "clarify_requested_at": c.ClarifyRequestedAt, "clarified_at": c.ClarifiedAt,
		"respond_by": c.RespondBy, "answer_by": c.AnswerBy, "answered_at": c.AnsweredAt, "created_at": c.CreatedAt, "expires_at": c.ExpiresAt,
		"purged": c.PurgedAt != nil, "hidden": c.HiddenAt != nil}
	if c.HiddenAt != nil || c.PurgedAt != nil {
		out["question"] = ""
		out["answer"] = ""
		out["video_public_id"] = nil
		out["clarify_body"] = nil
		out["clarification"] = nil
		return out
	}
	if c.VideoPublicID != nil {
		out["video_url"] = s.cloud.CoachingPlaybackURL(*c.VideoPublicID)
	}
	if c.AnswerVideoPublicID != nil {
		out["answer_video_url"] = s.cloud.CoachingPlaybackURL(*c.AnswerVideoPublicID)
	}
	return out
}
func (s *Server) listConsultationRoom(w http.ResponseWriter, r *http.Request) {
	room, role, ok := s.consultationRoomAccess(w, r)
	if !ok {
		return
	}
	params := db.ListRoomConsultationsParams{}
	if room.review != nil {
		params.ReviewRequestID = &room.id
	} else {
		params.ContractID = &room.id
	}
	rows, err := s.q.ListRoomConsultations(r.Context(), params)
	if err != nil {
		MapDBError(w, err)
		return
	}
	items := []map[string]any{}
	for _, c := range rows {
		if role != "requester" && c.Status == "draft" {
			continue
		}
		if role == "coach" && c.Status == "awaiting_payment" {
			continue
		}
		if role == "guardian" && !c.Minor {
			continue
		}
		item := s.consultationJSON(c)
		// フォーム添削の追加相談は運営者がコーチに委託するので、コーチには取引条件を見せる。
		if role == "coach" && room.review != nil && c.CoachID != nil {
			if err := s.attachConsultationWorkOrder(r.Context(), item, c); err != nil {
				MapDBError(w, err)
				return
			}
		}
		items = append(items, item)
	}
	price, _, requiresVideo, offerErr := s.consultationOffer(r.Context(), s.q, room)
	funding := "cash"
	if room.review != nil && offerErr == nil {
		funding, offerErr = reviewConsultationFunding(r.Context(), s.q, room)
	}
	reason := ""
	if offerErr != nil {
		if !writeTicketEligibilityReason(offerErr, &reason) {
			MapDBError(w, offerErr)
			return
		}
	}
	var coachID any
	if room.coach != nil {
		coachID = *room.coach
	}
	JSON(w, 200, map[string]any{"consultations": items, "role": role, "price_yen": price, "can_purchase": offerErr == nil && role == "requester",
		"block_reason": reason, "requires_video": requiresVideo, "coach_id": coachID, "funding": funding})
}

func (s *Server) validateConsultationContent(w http.ResponseWriter, r *http.Request, body, video string, owner uuid.UUID, existing *string) (*string, bool) {
	if utf8.RuneCountInString(body) > 2000 || strings.TrimSpace(body) == "" && video == "" {
		FieldError(w, map[string]string{"body": "質問または動画を入力してください（文章は2000文字まで）"})
		return nil, false
	}
	if looksLikeContact(body) {
		Error(w, 422, "contact_blocked", "外部の連絡先は送れません")
		return nil, false
	}
	if video == "" {
		return nil, true
	}
	if !cloudinary.ValidCoachingPublicID(video) {
		FieldError(w, map[string]string{"video_public_id": "動画を選び直してください"})
		return nil, false
	}
	if existing == nil || *existing != video {
		if !s.coachingVideoFree(w, r, video, "video_public_id") {
			return nil, false
		}
	}
	if !s.uploadIssuedToMe(w, r, owner, video, "video_public_id", "動画") {
		return nil, false
	}
	if err := s.cloud.CoachingVideoReady(r.Context(), video); err != nil {
		writeAssetReadyError(w, err, "動画")
		return nil, false
	}
	return &video, true
}
func (s *Server) saveConsultationDraft(w http.ResponseWriter, r *http.Request) {
	room, role, ok := s.consultationRoomAccess(w, r)
	if !ok {
		return
	}
	if role != "requester" {
		Error(w, 403, "forbidden", "相談する本人だけが下書きを保存できます")
		return
	}
	var req struct {
		RequestKey    uuid.UUID `json:"request_key"`
		Body          string    `json:"body"`
		VideoPublicID string    `json:"video_public_id"`
	}
	if err := DecodeJSON(r, &req); err != nil || req.RequestKey == uuid.Nil {
		FieldError(w, map[string]string{"body": "質問を保存し直してください"})
		return
	}
	req.Body = strings.TrimSpace(req.Body)
	previous, prevErr := s.q.GetConsultationByKey(r.Context(), db.GetConsultationByKeyParams{RequesterID: &room.owner, RequestKey: req.RequestKey})
	if prevErr != nil && !errors.Is(prevErr, pgx.ErrNoRows) {
		MapDBError(w, prevErr)
		return
	}
	var existing *string
	if prevErr == nil {
		existing = previous.VideoPublicID
	}
	video, ok := s.validateConsultationContent(w, r, req.Body, req.VideoPublicID, room.owner, existing)
	if !ok {
		return
	}
	var saved db.CoachingConsultation
	err := s.tx(r.Context(), func(q *db.Queries) error {
		if err := q.LockReviewRequester(r.Context(), room.owner); err != nil {
			return err
		}
		if err := q.LockConsultationRoot(r.Context(), room.id); err != nil {
			return err
		}
		fresh, err := s.consultationRoot(r.Context(), q, room.id, room.review != nil)
		if err != nil {
			return err
		}
		price, share, requiresVideo, err := s.consultationOffer(r.Context(), q, fresh)
		if err != nil {
			return err
		}
		if requiresVideo && video == nil {
			return &ticketRuleError{"video_required", "期間内の文章での確認は通常チャットで無料です。追加の動画を選んでください"}
		}
		owner, err := q.GetProfileByID(r.Context(), room.owner)
		if err != nil {
			return err
		}
		if err := s.ticketEligibility(r.Context(), q, owner); err != nil {
			return err
		}
		old, err := q.GetConsultationByKey(r.Context(), db.GetConsultationByKeyParams{RequesterID: &room.owner, RequestKey: req.RequestKey})
		if err == nil {
			parent, _ := consultationParent(old)
			if parent != room.id {
				return &ticketRuleError{"draft_conflict", "別の相談の操作IDは使えません"}
			}
			if old.Status != "draft" {
				sameVideo := old.VideoPublicID == nil && video == nil || old.VideoPublicID != nil && video != nil && *old.VideoPublicID == *video
				if old.Question != req.Body || !sameVideo {
					return &ticketRuleError{"already_submitted", "送信済みの質問は変更できません。部屋を読み直してください"}
				}
				saved = old
				return nil
			}
			if err := lockUnusedCoachingVideo(r.Context(), q, video, old.VideoPublicID); err != nil {
				return err
			}
			saved, err = q.UpdateConsultationDraft(r.Context(), db.UpdateConsultationDraftParams{ID: old.ID, Question: req.Body, VideoPublicID: video})
			return err
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err := lockUnusedCoachingVideo(r.Context(), q, video, nil); err != nil {
			return err
		}
		params := db.CreateConsultationDraftParams{ID: uuid.New(), RequesterID: &room.owner, CoachID: fresh.coach, RequestKey: req.RequestKey, Minor: fresh.minor,
			Funding: "cash", PriceYen: price, SharePercent: share, Question: req.Body, VideoPublicID: video}
		if room.review != nil {
			params.ReviewRequestID = &room.id
			if params.Funding, err = reviewConsultationFunding(r.Context(), q, fresh); err != nil {
				return err
			}
		} else {
			params.ContractID = &room.id
		}
		saved, err = q.CreateConsultationDraft(r.Context(), params)
		return err
	})
	if err != nil {
		writeCheckoutError(w, err)
		return
	}
	JSON(w, 200, map[string]any{"consultation": s.consultationJSON(saved)})
}

// すべての状態変更は親→相談の順にロックする。購入側は先に本人の支出ロックを取得する。
func (s *Server) lockConsultation(ctx context.Context, q *db.Queries, c db.CoachingConsultation) (db.CoachingConsultation, consultationRoom, error) {
	id, review := consultationParent(c)
	if err := q.LockConsultationRoot(ctx, id); err != nil {
		return c, consultationRoom{}, err
	}
	room, err := s.consultationRoot(ctx, q, id, review)
	if err != nil {
		return c, room, err
	}
	if room.contract != nil {
		ct, err := q.LockConsultationContract(ctx, id)
		if err != nil {
			return c, room, err
		}
		room.contract = &ct
		if ct.Status == "suspended" || ct.Status == "refunded" {
			return c, room, &ticketRuleError{"consultation_unavailable", "この契約は追加のやりとりが停止されています"}
		}
	}
	locked, err := q.LockConsultation(ctx, c.ID)
	return locked, room, err
}
func (s *Server) submitConsultation(w http.ResponseWriter, r *http.Request) {
	c, role, ok := s.consultationParty(w, r)
	if !ok {
		return
	}
	if role != "requester" {
		Error(w, 403, "forbidden", "相談する本人だけが送信できます")
		return
	}
	var saved db.CoachingConsultation
	err := s.tx(r.Context(), func(q *db.Queries) error {
		if err := q.LockReviewRequester(r.Context(), *c.RequesterID); err != nil {
			return err
		}
		current, room, err := s.lockConsultation(r.Context(), q, c)
		if err != nil {
			return err
		}
		saved = current
		if current.Status != "draft" {
			if current.Status == "awaiting_payment" || current.Status == "offered" || current.Status == "in_review" || current.Status == "answered" {
				return nil
			}
			return &ticketRuleError{"consultation_closed", "この相談は終了しています"}
		}
		_, _, videoRequired, err := s.consultationOffer(r.Context(), q, room)
		if err != nil {
			return err
		}
		if videoRequired && current.VideoPublicID == nil {
			return &ticketRuleError{"video_required", "追加の動画を選んでください"}
		}
		owner, err := q.GetProfileByID(r.Context(), *current.RequesterID)
		if err != nil {
			return err
		}
		if err := s.ticketEligibility(r.Context(), q, owner); err != nil {
			return err
		}
		if room.review != nil {
			// 下書きを保存してから残高が変わっていたら、送信時の残高で払い方を決め直す。
			funding, err := reviewConsultationFunding(r.Context(), q, room)
			if err != nil {
				return err
			}
			if funding != current.Funding {
				if current, err = q.SetConsultationDraftFunding(r.Context(), db.SetConsultationDraftFundingParams{ID: current.ID, Funding: funding}); err != nil {
					return err
				}
				saved = current
			}
		}
		if current.Funding == "ticket" {
			if err := s.reserveConsultationTickets(r.Context(), q, owner, current); err != nil {
				return err
			}
			saved, err = q.SubmitTicketConsultation(r.Context(), current.ID)
			return err
		}
		if !s.stripe.Configured() {
			return &ticketRuleError{"payment_unavailable", "追加相談の決済は準備中です"}
		}
		if current.Minor {
			msg, err := s.coachingMonthlyBlock(r.Context(), q, *current.RequesterID, uuid.Nil, uuid.Nil, uuid.Nil, current.PriceYen, current.ID)
			if err != nil {
				return err
			}
			if msg != "" {
				return &ticketRuleError{"monthly_limit", msg}
			}
		}
		saved, err = q.StartCashConsultationPayment(r.Context(), current.ID)
		return err
	})
	if err != nil {
		writeCheckoutError(w, err)
		return
	}
	if saved.Funding == "cash" && saved.Status == "awaiting_payment" && !saved.Minor {
		url, err := s.consultationCheckout(r.Context(), saved)
		if err != nil {
			writeCheckoutError(w, err)
			return
		}
		JSON(w, 200, map[string]any{"consultation": s.consultationJSON(saved), "checkout_url": url})
		return
	}
	if saved.Status == "offered" {
		s.notifyConsultation(r.Context(), saved, true, "追加相談が届きました")
	}
	JSON(w, 200, map[string]any{"consultation": s.consultationJSON(saved)})
}
func (s *Server) reserveConsultationTickets(ctx context.Context, q *db.Queries, p db.Profile, c db.CoachingConsultation) error {
	if _, err := q.GetConsultationTicketReservation(ctx, &c.ID); err == nil {
		return nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	lots, err := q.ListTicketLots(ctx, &p.ID)
	if err != nil {
		return err
	}
	if len(lots) == 0 {
		return &ticketRuleError{"insufficient_tickets", "チケットが不足しています。下書きは保存済みです。購入後にこの部屋から送信できます"}
	}
	reservation, err := q.CreateConsultationTicketReservation(ctx, db.CreateConsultationTicketReservationParams{ID: uuid.New(), OwnerID: &p.ID, ConsultationID: &c.ID, SharePercent: c.SharePercent})
	if err != nil {
		return err
	}
	lot := lots[0]
	if err := q.CreateTicketAllocation(ctx, db.CreateTicketAllocationParams{ReservationID: reservation.ID, LotID: lot.ID, Quantity: 1}); err != nil {
		return err
	}
	return moveTickets(ctx, q, lot.ID, "reserve-"+reservation.ID.String()+"-"+lot.ID.String(), "reserve", -1, 1, 0, 0)
}
func (s *Server) notifyConsultation(ctx context.Context, c db.CoachingConsultation, toCoach bool, title string) {
	parent, review := consultationParent(c)
	room := consultationRoom{id: parent}
	if !review {
		room.contract = &db.CoachingContract{}
	}
	target := c.RequesterID
	if toCoach {
		target = c.CoachID
	}
	if target == nil {
		return
	}
	s.notify(ctx, *target, notifyCoaching, push.Message{Title: title, Body: "追加相談の部屋を開いて確認してください", URL: room.path(toCoach), Tag: "consultation-" + c.ID.String()}, 24*time.Hour)
}
func (s *Server) actConsultation(w http.ResponseWriter, r *http.Request) {
	action := chi.URLParam(r, "action")
	// 回答・質問・補足はやりとりにあたるので、届出が済むまで止める。取り消しは止めない。
	if (action == "answer" || action == "clarify" || action == "clarification") && !s.cfg.CoachingMessaging {
		Error(w, http.StatusConflict, "messaging_closed", "メッセージのやりとりは準備中です")
		return
	}
	c, role, ok := s.consultationParty(w, r)
	if !ok {
		return
	}
	ownerAction := action == "clarify" || action == "cancel"
	if ownerAction && role != "requester" || !ownerAction && role != "coach" {
		Error(w, 403, "forbidden", "この操作はできません")
		return
	}
	var req struct {
		Body          string `json:"body"`
		VideoPublicID string `json:"video_public_id"`
	}
	if action == "answer" || action == "clarify" || action == "clarification" {
		if err := DecodeJSON(r, &req); err != nil {
			Error(w, 400, "validation_error", "JSONが不正です")
			return
		}
		req.Body = strings.TrimSpace(req.Body)
	}
	var video *string
	if action == "answer" {
		var valid bool
		video, valid = s.validateConsultationContent(w, r, req.Body, req.VideoPublicID, *c.CoachID, nil)
		if !valid {
			return
		}
	}
	if action == "clarify" || action == "clarification" {
		if req.Body == "" || utf8.RuneCountInString(req.Body) > 2000 || looksLikeContact(req.Body) {
			FieldError(w, map[string]string{"body": "連絡先を含まない文章を1〜2000文字で入力してください"})
			return
		}
	}
	var saved db.CoachingConsultation
	err := s.tx(r.Context(), func(q *db.Queries) error {
		current, room, err := s.lockConsultation(r.Context(), q, c)
		if err != nil {
			return err
		}
		if current.HiddenAt != nil || current.PurgedAt != nil {
			return &ticketRuleError{"consultation_closed", "この相談は利用できません"}
		}
		if role == "coach" {
			cp, err := q.GetCoachProfile(r.Context(), *current.CoachID)
			if err != nil {
				return err
			}
			if cp.Status != "approved" || !s.payoutsReady(cp) {
				return &ticketRuleError{"coach_unavailable", "承認済みのコーチだけが対応できます"}
			}
		}
		switch action {
		case "accept":
			saved, err = q.AcceptConsultation(r.Context(), db.AcceptConsultationParams{ID: c.ID, CoachID: c.CoachID})
			// フォーム添削の追加相談は、引き受けた時点の取引条件を書き留める(フリーランス保護法3条の明示)。
			// サポートプランの追加相談はコーチが販売者なので、要否は弁護士の回答(Q9)を待つ。
			if err == nil && saved.ReviewRequestID != nil {
				name, e := coachDisplayName(r.Context(), q, *saved.CoachID)
				if e != nil {
					return e
				}
				err = saveWorkOrder(r.Context(), q, "consultation", nil, &saved.ID, *saved.CoachID, consultationWorkOrderTerms(saved, name, time.Now()))
			}
		case "decline":
			if current.Status != "offered" {
				return &ticketRuleError{"invalid_state", "受諾前の相談だけ辞退できます"}
			}
			saved, err = q.CloseConsultation(r.Context(), db.CloseConsultationParams{ID: c.ID, Status: "declined"})
			if err == nil {
				err = s.releaseConsultationFunding(r.Context(), q, saved)
			}
		case "cancel":
			if current.Status != "draft" && current.Status != "awaiting_payment" {
				return &ticketRuleError{"invalid_state", "送信済みの相談は取り下げられません"}
			}
			saved, err = q.CloseConsultation(r.Context(), db.CloseConsultationParams{ID: c.ID, Status: "cancelled"})
		case "answer":
			if err := lockUnusedCoachingVideo(r.Context(), q, video, nil); err != nil {
				return err
			}
			saved, err = q.AnswerConsultation(r.Context(), db.AnswerConsultationParams{ID: c.ID, Answer: req.Body, AnswerVideoPublicID: video})
			if err == nil && current.Funding == "ticket" {
				reservation, e := q.GetConsultationTicketReservation(r.Context(), &current.ID)
				if e != nil {
					return e
				}
				if e := settleReviewTickets(r.Context(), q, reservation, true); e != nil {
					return e
				}
			}
			if err == nil {
				err = q.RecordConsultationEarnings(r.Context(), current.ID)
			}
		case "clarify":
			owner, profileErr := q.GetProfileByID(r.Context(), room.owner)
			if profileErr != nil {
				return profileErr
			}
			if err := s.ticketEligibility(r.Context(), q, owner); err != nil {
				return err
			}
			saved, err = q.RequestConsultationClarification(r.Context(), db.RequestConsultationClarificationParams{ID: c.ID, ClarifyBody: &req.Body})
		case "clarification":
			saved, err = q.AnswerConsultationClarification(r.Context(), db.AnswerConsultationClarificationParams{ID: c.ID, Clarification: &req.Body})
		default:
			return &ticketRuleError{"invalid_action", "操作が不正です"}
		}
		return err
	})
	if err != nil {
		writeCheckoutError(w, err)
		return
	}
	if action == "answer" || action == "clarification" || action == "decline" {
		s.notifyConsultation(r.Context(), saved, false, "追加相談が更新されました")
	}
	if action == "clarify" {
		s.notifyConsultation(r.Context(), saved, true, "回答への確認質問が届きました")
	}
	JSON(w, 200, map[string]any{"consultation": s.consultationJSON(saved)})
}
func (s *Server) coachConsultations(w http.ResponseWriter, r *http.Request) {
	cp, ok := s.approvedCoach(w, r)
	if !ok {
		return
	}
	rows, err := s.q.ListCoachConsultations(r.Context(), &cp.ProfileID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if rows == nil {
		rows = []db.ListCoachConsultationsRow{}
	}
	JSON(w, 200, map[string]any{"consultations": rows})
}
