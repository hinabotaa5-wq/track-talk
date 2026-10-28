package httpx

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/jackc/pgx/v5"
)

// 18歳未満の利用者のパーソナル指導に、保護者がリンク（ログイン不要）から参加する。
// コーチが承諾したら、保護者がその契約ごとに同意して期間が始まる。期間中は参加者として読み書きと通報ができる。
// 保護者にはメッセージごとの通知はしない。

// guardianMinor は、保護者のリンクから子の ID を返す。添削と同じ同意が済んでいないと使えない。
func (s *Server) guardianMinor(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	a, ok := s.guardianByToken(w, r)
	if !ok {
		return uuid.Nil, false
	}
	consent := a.consent
	if consent.Status != "consented" {
		Error(w, http.StatusConflict, "guardian_not_consented", "先に、このページの上の同意を済ませてください")
		return uuid.Nil, false
	}
	return consent.MinorID, true
}

// guardianContract は、保護者のリンクから子の契約を読む。子のものでなければ 404。
func (s *Server) guardianContract(w http.ResponseWriter, r *http.Request) (db.CoachingContract, bool) {
	minor, ok := s.guardianMinor(w, r)
	if !ok {
		return db.CoachingContract{}, false
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return db.CoachingContract{}, false
	}
	ct, err := s.q.GetCoachingContract(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (ct.RequesterID != minor || !ct.Minor)) {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return db.CoachingContract{}, false
	}
	if err != nil {
		MapDBError(w, err)
		return db.CoachingContract{}, false
	}
	return ct, true
}

func (s *Server) listGuardianContracts(w http.ResponseWriter, r *http.Request) {
	minor, ok := s.guardianMinor(w, r)
	if !ok {
		return
	}
	rows, err := s.q.ListGuardianContracts(r.Context(), minor)
	if err != nil {
		MapDBError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		c := s.contractJSON(db.CoachingContract{
			PreferredDates: row.PreferredDates,
			ID:             row.ID, PlanID: row.PlanID, CoachID: row.CoachID, RequesterID: row.RequesterID, PlanKind: row.PlanKind,
			PlanTitle: row.PlanTitle, DurationDays: row.DurationDays, PriceYen: row.PriceYen, Event: row.Event, Goal: row.Goal,
			CurrentRecord: row.CurrentRecord, Message: row.Message, Minor: row.Minor, Status: row.Status, RespondBy: row.RespondBy,
			RespondedAt: row.RespondedAt, StartsAt: row.StartsAt, EndsAt: row.EndsAt, CreatedAt: row.CreatedAt,
			StartBy: row.StartBy, SuspendedAt: row.SuspendedAt, PaymentRequired: row.PaymentRequired, MessagesPurgedAt: row.MessagesPurgedAt,
			VideoReviews: row.VideoReviews, MenuUpdates: row.MenuUpdates, ReplyFormats: row.ReplyFormats,
			StartOnFirstVideo: row.StartOnFirstVideo, ClockStartedAt: row.ClockStartedAt,
			CompetitionName: row.CompetitionName, CompetitionDate: row.CompetitionDate, SupportStartDate: row.SupportStartDate, CallCount: row.CallCount, CallMinutes: row.CallMinutes, FulfillmentVersion: row.FulfillmentVersion, FulfillmentState: row.FulfillmentState,
			ServiceType: row.ServiceType, PlanDescription: row.PlanDescription, AdditionalConsultationPriceYen: row.AdditionalConsultationPriceYen,
		})
		c["coach"] = map[string]any{"id": row.CoachID, "nickname": row.CoachNickname, "photo_url": s.coachPhotoURL(row.CoachPhotoPublicID)}
		c["guardian_consented"] = false
		if row.Status == "accepted" {
			consented, err := s.q.HasGuardianContractConsent(r.Context(), row.ID)
			if err != nil {
				MapDBError(w, err)
				return
			}
			c["guardian_consented"] = consented
		}
		out = append(out, c)
	}
	JSON(w, http.StatusOK, map[string]any{"contracts": out})
}

// guardianContractJSON は、保護者に見せる契約。保護者がこの契約に同意したかと、支払いをつける。
func (s *Server) guardianContractJSON(ctx context.Context, id uuid.UUID) (map[string]any, error) {
	row, err := s.q.GetCoachingContractDetail(ctx, id)
	if err != nil {
		return nil, err
	}
	c := s.contractJSON(db.CoachingContract{
		PreferredDates: row.PreferredDates,
		ID:             row.ID, PlanID: row.PlanID, CoachID: row.CoachID, RequesterID: row.RequesterID, PlanKind: row.PlanKind,
		PlanTitle: row.PlanTitle, DurationDays: row.DurationDays, PriceYen: row.PriceYen, Event: row.Event, Goal: row.Goal,
		CurrentRecord: row.CurrentRecord, Message: row.Message, Minor: row.Minor, Status: row.Status, RespondBy: row.RespondBy,
		RespondedAt: row.RespondedAt, StartsAt: row.StartsAt, EndsAt: row.EndsAt, CreatedAt: row.CreatedAt,
		StartBy: row.StartBy, SuspendedAt: row.SuspendedAt, PaymentRequired: row.PaymentRequired, MessagesPurgedAt: row.MessagesPurgedAt,
		VideoReviews: row.VideoReviews, MenuUpdates: row.MenuUpdates, ReplyFormats: row.ReplyFormats,
		StartOnFirstVideo: row.StartOnFirstVideo, ClockStartedAt: row.ClockStartedAt,
		CompetitionName: row.CompetitionName, CompetitionDate: row.CompetitionDate, SupportStartDate: row.SupportStartDate, CallCount: row.CallCount, CallMinutes: row.CallMinutes, FulfillmentVersion: row.FulfillmentVersion, FulfillmentState: row.FulfillmentState,
		ServiceType: row.ServiceType, PlanDescription: row.PlanDescription, AdditionalConsultationPriceYen: row.AdditionalConsultationPriceYen,
	})
	c["role"] = "guardian"
	c["coach"] = map[string]any{"id": row.CoachID, "nickname": row.CoachNickname, "photo_url": s.coachPhotoURL(row.CoachPhotoPublicID)}
	c["requester"] = map[string]any{"nickname": row.RequesterNickname}
	consented, err := s.q.HasGuardianContractConsent(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	c["guardian_consented"] = consented
	if err := s.attachPayment(ctx, c, row.ID); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Server) writeGuardianContract(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	c, err := s.guardianContractJSON(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"contract": c})
}

func (s *Server) getGuardianContract(w http.ResponseWriter, r *http.Request) {
	ct, ok := s.guardianContract(w, r)
	if !ok {
		return
	}
	s.writeGuardianContract(w, r, ct.ID)
}

// consentGuardianContract は、保護者がこの契約に同意する。利用者・コーチと同じ2つに同意すると、その時から期間が始まる。
func (s *Server) consentGuardianContract(w http.ResponseWriter, r *http.Request) {
	ct, ok := s.guardianContract(w, r)
	if !ok {
		return
	}
	var req contractConsentReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if fields := req.fields(); len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	if ct.PaymentRequired {
		s.consentAndCheckoutGuardian(w, r, ct)
		return
	}
	var started db.CoachingContract
	err := s.tx(r.Context(), func(q *db.Queries) error {
		var err error
		started, err = q.GuardianStartCoachingContract(r.Context(), db.GuardianStartCoachingContractParams{ID: ct.ID, RequesterID: ct.RequesterID})
		if err != nil {
			return err
		}
		return q.InsertContractConsent(r.Context(), db.InsertContractConsentParams{
			ContractID: ct.ID, Party: "guardian", ProfileID: nil, Version: contractConsentVersion,
		})
	})
	if errors.Is(err, pgx.ErrNoRows) {
		if ct.Status == "accepted" {
			Error(w, http.StatusConflict, "conflict", "同意の期限を過ぎました")
		} else {
			Error(w, http.StatusConflict, "conflict", "この申し込みは同意を待っていません")
		}
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	msg := push.Message{
		Title: "パーソナル指導が始まりました",
		Body:  "保護者が「" + started.PlanTitle + "」に同意しました",
		URL:   "/coaching/personal/" + started.ID.String(),
		Tag:   "contract-" + started.ID.String(),
	}
	s.notify(r.Context(), started.RequesterID, notifyCoaching, msg, 24*time.Hour)
	s.notify(r.Context(), started.CoachID, notifyCoaching, msg, 24*time.Hour)
	s.writeGuardianContract(w, r, started.ID)
}

func (s *Server) declineGuardianContract(w http.ResponseWriter, r *http.Request) {
	ct, ok := s.guardianContract(w, r)
	if !ok {
		return
	}
	declined, err := s.q.GuardianDeclineCoachingContract(r.Context(), db.GuardianDeclineCoachingContractParams{ID: ct.ID, RequesterID: ct.RequesterID})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "conflict", "この申し込みは同意を待っていません")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	msg := push.Message{
		Title: "パーソナル指導は始まりませんでした",
		Body:  "保護者が「" + declined.PlanTitle + "」に同意しませんでした",
		URL:   "/coaching/personal/" + declined.ID.String(),
		Tag:   "contract-" + declined.ID.String(),
	}
	s.notify(r.Context(), declined.RequesterID, notifyCoaching, msg, 24*time.Hour)
	msg.URL = "/coach"
	s.notify(r.Context(), declined.CoachID, notifyCoaching, msg, 24*time.Hour)
	s.writeGuardianContract(w, r, declined.ID)
}

func (s *Server) listGuardianMessages(w http.ResponseWriter, r *http.Request) {
	ct, ok := s.guardianContract(w, r)
	if !ok {
		return
	}
	s.writeCoachingMessages(w, r, ct, uuid.Nil, "guardian")
}

func (s *Server) createGuardianMessage(w http.ResponseWriter, r *http.Request) {
	ct, ok := s.guardianContract(w, r)
	if !ok {
		return
	}
	s.sendCoachingMessage(w, r, ct, nil, "guardian")
}

// reportGuardianMessage は、保護者がやりとり欄の1通を通報する。保護者が送ったものは通報できない。
func (s *Server) reportGuardianMessage(w http.ResponseWriter, r *http.Request) {
	minor, ok := s.guardianMinor(w, r)
	if !ok {
		return
	}
	mid, ok := parseUUIDParam(w, r, "mid")
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
		Detail string `json:"detail"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	fields := map[string]string{}
	if e := ReportReason(req.Reason); e != "" {
		fields["reason"] = e
	}
	if e := ReportDetail(req.Detail); e != "" {
		fields["detail"] = e
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	m, err := s.q.GetCoachingMessageContract(r.Context(), mid)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (m.RequesterID != minor || !m.Minor || m.SenderRole == "guardian")) {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	_, err = s.q.CreateGuardianReport(r.Context(), db.CreateGuardianReportParams{
		ID:         uuid.New(),
		ReporterID: minor,
		TargetID:   mid,
		Reason:     req.Reason,
		Detail:     strings.TrimSpace(req.Detail),
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	s.notifyAdminsMinorReport(r.Context())
	// 通報の行には子の ID が入るので返さない。
	JSON(w, http.StatusCreated, map[string]any{"reported": true})
}

// consentAndCheckoutGuardian は、支払いがある契約で、保護者が同意して支払い画面へ進む。
// 期間は支払いが済んだ時から始まる。同意は1回だけ記録し、支払い画面は開き直せる。
func (s *Server) consentAndCheckoutGuardian(w http.ResponseWriter, r *http.Request, ct db.CoachingContract) {
	if !checkoutOpen(w, ct) {
		return
	}
	consented, err := s.q.HasGuardianContractConsent(r.Context(), ct.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if !consented {
		if err := s.q.InsertContractConsent(r.Context(), db.InsertContractConsentParams{
			ContractID: ct.ID, Party: "guardian", ProfileID: nil, Version: contractConsentVersion,
		}); err != nil {
			MapDBError(w, err)
			return
		}
	}
	url, err := s.guardianCheckoutURL(r.Context(), ct)
	if err != nil {
		writeCheckoutError(w, err)
		return
	}
	c, err := s.guardianContractJSON(r.Context(), ct.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"contract": c, "checkout_url": url})
}
