package httpx

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
	"golang.org/x/text/width"
)

const referralRewardTerms = "特典チケット：付与日から6か月有効／付与された本人だけが使えます／払戻しはできません"

func normalizeReferralCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(width.Narrow.String(code)))
}

func usableReferralCode(ctx context.Context, q *db.Queries, buyer uuid.UUID, input string) (*string, error) {
	code := normalizeReferralCode(input)
	if len(code) != 6 {
		return nil, nil
	}
	valid, err := q.GetUsableCoachingReferral(ctx, db.GetUsableCoachingReferralParams{Code: code, Buyer: buyer})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &valid, err
}

func recordCoachingReferral(ctx context.Context, q *db.Queries, buyer, ticketID, paymentID *uuid.UUID, code *string) error {
	if buyer == nil {
		return nil
	}
	if err := q.IssueCoachingReferral(ctx, *buyer); err != nil || code == nil {
		return err
	}
	recipient, err := q.CountCoachingReferral(ctx, db.CountCoachingReferralParams{Buyer: *buyer, Code: *code, TicketID: ticketID, PaymentID: paymentID})
	if err != nil || recipient == "" {
		return err
	}
	id, err := uuid.Parse(recipient)
	if err != nil {
		return err
	}
	if wanted, err := notificationWanted(ctx, q, id, notifyCoaching, true); err != nil || !wanted {
		return err
	}
	title, body, url := "紹介特典：無料チケットを1枚プレゼントしました", referralRewardTerms, "/me"
	return q.InsertMessageNotification(ctx, db.InsertMessageNotificationParams{
		ID: uuid.New(), RecipientID: id, Title: &title, Body: &body, Url: &url,
	})
}

func (s *Server) myCoachingReferral(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	progress, err := s.q.GetCoachingReferralProgress(r.Context(), p.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		JSON(w, http.StatusOK, map[string]any{"referral": nil})
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"referral": progress})
}

func (s *Server) checkCoachingReferral(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "紹介IDを確認してください")
		return
	}
	code, err := usableReferralCode(r.Context(), s.q, p.ID, req.Code)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]bool{"valid": code != nil})
}

func (s *Server) adminCoachingReferrals(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	rows, err := s.q.AdminCoachingReferralOverview(r.Context(), normalizeReferralCode(r.URL.Query().Get("code")))
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"referrals": rows})
}
