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

// recordCoachingReferral は、確定した対象購入1件を紹介の周に反映する。入力された紹介IDで
// 数えたうえで、その購入に有効カウントが紐付かなかったときだけ購入者の起点購入として
// 紹介IDを発行する（本体 #686：起点とカウントの分離）。
func recordCoachingReferral(ctx context.Context, q *db.Queries, buyer, ticketID, paymentID *uuid.UUID, code *string) error {
	if buyer == nil {
		return nil
	}
	recipient, err := q.RecordCoachingReferral(ctx, db.RecordCoachingReferralParams{Buyer: *buyer, Code: code, TicketID: ticketID, PaymentID: paymentID})
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
	title, body, url := "紹介特典：¥500クーポンをプレゼントしました", referralRewardTerms, "/me/coaching/tickets"
	role := "player"
	return q.InsertMessageNotification(ctx, db.InsertMessageNotificationParams{
		ID: uuid.New(), RecipientID: id, Title: &title, Body: &body, Url: &url, CoachingRole: &role,
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
	// 特典の種類（本体 #1031）：coupon＝¥500 クーポン、ticket＝発行済みの特典チケット。まだ付与していなければ null。
	var kind, expires any
	switch {
	case progress.CouponExpiresAt != nil:
		kind, expires = "coupon", progress.CouponExpiresAt
	case progress.TicketExpiresAt != nil:
		kind, expires = "ticket", progress.TicketExpiresAt
	}
	JSON(w, http.StatusOK, map[string]any{"referral": map[string]any{
		"code": progress.Code, "active": progress.Active, "friend_count": progress.FriendCount,
		"granted_at": progress.GrantedAt, "expires_at": expires, "reward_kind": kind,
	}})
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
	// already_counted：紹介者にすでに数えられている購入者には、その旨を伝える（本体 #686）。
	counted := false
	if code == nil {
		if normalized := normalizeReferralCode(req.Code); len(normalized) == 6 {
			if counted, err = s.q.CoachingReferralAlreadyCounted(r.Context(), db.CoachingReferralAlreadyCountedParams{Code: normalized, Buyer: p.ID}); err != nil {
				MapDBError(w, err)
				return
			}
		}
	}
	JSON(w, http.StatusOK, map[string]bool{"valid": code != nil, "already_counted": counted})
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
