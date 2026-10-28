package httpx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/stripe"
	"github.com/jackc/pgx/v5"
)

// 購入操作は相談ごとに固定する。通知が先に届いても同じレコードで照合できる。
func (s *Server) consultationCheckout(ctx context.Context, initial db.CoachingConsultation) (string, error) {
	var c db.CoachingConsultation
	if !s.stripe.Configured() {
		return "", &ticketRuleError{"payment_unavailable", "追加相談の決済は準備中です"}
	}
	err := s.tx(ctx, func(q *db.Queries) error {
		if initial.RequesterID == nil {
			return pgx.ErrNoRows
		}
		if err := q.LockReviewRequester(ctx, *initial.RequesterID); err != nil {
			return err
		}
		current, room, err := s.lockConsultation(ctx, q, initial)
		if err != nil {
			return err
		}
		if current.Funding != "cash" || current.Status != "awaiting_payment" || !current.ExpiresAt.After(time.Now()) {
			return &ticketRuleError{"payment_closed", "この相談は支払いを待っていません"}
		}
		if _, _, _, err := s.consultationOffer(ctx, q, room); err != nil {
			return err
		}
		owner, err := q.GetProfileByID(ctx, *current.RequesterID)
		if err != nil {
			return err
		}
		if err := s.ticketEligibility(ctx, q, owner); err != nil {
			return err
		}
		if current.Minor && current.ContractID == nil {
			// 添削のあとの追加相談は、本人が自分で払う。保護者の有効な同意と月の上限を、このロックの中で確かめる。
			block, err := s.minorSelfPayBlock(ctx, q, owner.ID, minorPurchase{refType: "consultation", coachID: current.CoachID, price: current.PriceYen}, uuid.Nil, current.ID)
			if err != nil {
				return err
			}
			if block != nil {
				return block
			}
		} else if current.Minor {
			consented, err := q.HasGuardianContractConsent(ctx, *current.ContractID)
			if err != nil {
				return err
			}
			if !consented {
				return &ticketRuleError{"guardian_required", "先に契約への保護者同意が必要です"}
			}
			msg, err := s.coachingMonthlyBlock(ctx, q, owner.ID, uuid.Nil, uuid.Nil, uuid.Nil, current.PriceYen, current.ID)
			if err != nil {
				return err
			}
			if msg != "" {
				return &ticketRuleError{"monthly_limit", msg}
			}
		}
		if current.SessionExpiresAt == nil {
			expiry := time.Now().Add(time.Hour)
			if current.ExpiresAt.Before(expiry) {
				expiry = current.ExpiresAt
			}
			current, err = q.SetConsultationSessionExpiry(ctx, db.SetConsultationSessionExpiryParams{ID: current.ID, SessionExpiresAt: &expiry})
			if err != nil {
				return err
			}
		}
		c = current
		return nil
	})
	if err != nil {
		return "", err
	}
	if !c.SessionExpiresAt.After(time.Now().Add(checkoutMinLife)) {
		return "", &ticketRuleError{"payment_expired", "支払いの期限を過ぎました。相談を取り下げ、新しく作成してください"}
	}
	if c.CheckoutUrl != nil {
		return *c.CheckoutUrl, nil
	}
	path := "/coaching/personal/"
	if c.ContractID != nil {
		path += c.ContractID.String()
	} else if c.ReviewRequestID != nil {
		path = "/coaching/requests/" + c.ReviewRequestID.String()
	}
	if c.Minor && c.ContractID != nil {
		path = "/guardian/paid"
	}
	page := s.cfg.WebBaseURL + path
	sess, err := s.stripe.CreateCheckoutSession(ctx, stripe.CheckoutParams{Name: "追加相談（質問1件と回答1件）", AmountYen: int(c.PriceYen), SuccessURL: page + "?consultation=" + c.ID.String(), CancelURL: page + "?cancelled=1", Reference: c.ID.String(), Metadata: map[string]string{"kind": "consultation", "consultation_id": c.ID.String()}, ExpiresAt: *c.SessionExpiresAt, PaymentMethodTypes: checkoutPaymentMethods, IdempotencyKey: "consultation-checkout-" + c.ID.String()})
	if err != nil {
		return "", fmt.Errorf("%w: %w", errStripe, err)
	}
	if err := s.q.SaveConsultationCheckout(ctx, db.SaveConsultationCheckoutParams{ID: c.ID, CheckoutSessionID: &sess.ID, CheckoutUrl: &sess.URL}); err != nil {
		return "", err
	}
	return sess.URL, nil
}
func (s *Server) checkoutConsultation(w http.ResponseWriter, r *http.Request) {
	c, role, ok := s.consultationParty(w, r)
	if !ok {
		return
	}
	if role != consultationPayer(c) {
		Error(w, 403, "guardian_pays", "未成年のパーソナル指導の追加相談は保護者のページからお支払いください")
		return
	}
	url, err := s.consultationCheckout(r.Context(), c)
	if s.writeMinorBlockError(w, r, err) {
		return
	}
	if err != nil {
		writeCheckoutError(w, err)
		return
	}
	JSON(w, 200, map[string]any{"checkout_url": url})
}
func (s *Server) completeConsultationCheckout(ctx context.Context, sess stripe.CheckoutSession) error {
	if sess.PaymentStatus != "paid" || sess.PaymentIntent == "" {
		return nil
	}
	id, err := uuid.Parse(sess.Metadata["consultation_id"])
	if err != nil {
		return errors.New("consultation checkout missing id")
	}
	initial, err := s.q.GetConsultation(ctx, id)
	if err != nil {
		return err
	}
	var saved db.CoachingConsultation
	var notice *db.GuardianNotice
	notify := false
	err = s.tx(ctx, func(q *db.Queries) error {
		root, _ := consultationParent(initial)
		if err := q.LockConsultationRoot(ctx, root); err != nil {
			return err
		}
		c, err := q.LockConsultation(ctx, id)
		if err != nil {
			return err
		}
		if c.Funding != "cash" || sess.ClientReferenceID != id.String() || sess.AmountTotal != int(c.PriceYen) || sess.Currency != "jpy" || (c.CheckoutSessionID != nil && *c.CheckoutSessionID != sess.ID) {
			return errors.New("consultation checkout mismatch")
		}
		if c.PaymentState != "unpaid" {
			return nil
		}
		c, err = q.MarkConsultationPaid(ctx, db.MarkConsultationPaidParams{ID: id, PaymentIntentID: &sess.PaymentIntent, CheckoutSessionID: &sess.ID})
		if err != nil {
			return err
		}
		parent, review := consultationParent(c)
		room, rootErr := s.consultationRoot(ctx, q, parent, review)
		valid := rootErr == nil && c.RequesterID != nil && c.CoachID != nil && c.HiddenAt == nil
		if valid {
			cp, e := q.GetCoachProfile(ctx, *c.CoachID)
			if e != nil && !errors.Is(e, pgx.ErrNoRows) {
				return e
			}
			valid = e == nil && cp.Status == "approved"
			if room.contract != nil && (room.contract.Status == "suspended" || room.contract.Status == "refunded") {
				valid = false
			}
		}
		if valid && c.Minor {
			consent, e := q.GetGuardianConsentByMinor(ctx, *c.RequesterID)
			if e != nil && !errors.Is(e, pgx.ErrNoRows) {
				return e
			}
			valid = e == nil && consent.Status == "consented"
		}
		if rootErr != nil && !errors.Is(rootErr, pgx.ErrNoRows) {
			return rootErr
		}
		if !valid && c.Status == "offered" {
			c, err = q.CloseConsultation(ctx, db.CloseConsultationParams{ID: id, Status: "expired"})
			if err != nil {
				return err
			}
		}
		saved = c
		if c.Status != "offered" {
			return s.releaseConsultationFunding(ctx, q, c)
		}
		notify = true
		if c.Minor && c.ContractID == nil {
			// 添削のあとの追加相談を18歳未満が自分で払ったときは、保護者への購入の通知を予約する。
			name, e := coachDisplayName(ctx, q, *c.CoachID)
			if e != nil && !errors.Is(e, pgx.ErrNoRows) {
				return e
			}
			notice, err = queueGuardianPurchaseNotice(ctx, q, *c.RequesterID, "consultation", c.ID, c.PriceYen, name, c.PaymentIntentID)
			return err
		}
		return nil
	})
	if err == nil && notify {
		if notice != nil {
			s.sendGuardianNotice(ctx, *notice)
		}
		s.notifyConsultation(ctx, saved, true, "追加相談が届きました")
	}
	return err
}

// consultationPayer は、追加相談を払う人。18歳未満のパーソナル指導（保護者が契約・支払いの当事者）の相談は保護者、
// それ以外（大人の相談と、18歳未満の添削のあとの相談）は本人。
func consultationPayer(c db.CoachingConsultation) string {
	if c.Minor && c.ContractID != nil {
		return "guardian"
	}
	return "requester"
}
func (s *Server) releaseConsultationFunding(ctx context.Context, q *db.Queries, c db.CoachingConsultation) error {
	if c.Funding == "ticket" {
		reservation, err := q.GetConsultationTicketReservation(ctx, &c.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		return settleReviewTickets(ctx, q, reservation, false)
	}
	if c.PaymentState != "paid" {
		return nil
	}
	refunds, err := q.ListConsultationRefunds(ctx, c.ID)
	if err != nil {
		return err
	}
	for _, r := range refunds {
		if r.Status != "failed" {
			return nil
		}
	}
	if _, err := q.CreateConsultationRefund(ctx, db.CreateConsultationRefundParams{ID: uuid.New(), ConsultationID: c.ID}); err != nil {
		return err
	}
	return q.SetConsultationPaymentState(ctx, db.SetConsultationPaymentStateParams{ID: c.ID, PaymentState: "refund_pending"})
}
func (s *Server) processConsultationRefund(ctx context.Context, id uuid.UUID) error {
	return s.tx(ctx, func(q *db.Queries) error {
		refund, err := q.LockConsultationRefund(ctx, id)
		if err != nil {
			return err
		}
		if refund.Status != "pending" {
			return nil
		}
		c, err := q.LockConsultation(ctx, refund.ConsultationID)
		if err != nil {
			return err
		}
		if c.PaymentIntentID == nil || c.Status == "answered" {
			return errors.New("invalid consultation refund")
		}
		operation := "consultation-" + id.String()
		remote, err := s.stripe.FindTicketRefund(ctx, *c.PaymentIntentID, operation)
		if err != nil {
			return err
		}
		if remote == nil {
			created, err := s.stripe.CreateRefundWithMetadata(ctx, *c.PaymentIntentID, int(c.PriceYen), operation)
			if err != nil {
				return err
			}
			remote = &created
		}
		if remote.ID == "" || remote.Amount != int(c.PriceYen) {
			return errors.New("consultation refund amount mismatch")
		}
		status := "pending"
		switch remote.Status {
		case "succeeded":
			status = "succeeded"
			if err := q.SetConsultationPaymentState(ctx, db.SetConsultationPaymentStateParams{ID: c.ID, PaymentState: "refunded"}); err != nil {
				return err
			}
		case "failed", "canceled":
			status = "failed"
		}
		return q.SaveConsultationRefund(ctx, db.SaveConsultationRefundParams{ID: id, Status: status, StripeRefundID: &remote.ID})
	})
}
func (s *Server) settleConsultationRefunds(ctx context.Context) (int, error) {
	if !s.stripe.Configured() {
		return 0, nil
	}
	rows, err := s.q.ListPendingConsultationRefunds(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, row := range rows {
		if err := s.processConsultationRefund(ctx, row.ID); err != nil {
			slog.Error("consultation refund", "refund", row.ID, "err", err)
			if err := s.q.TouchConsultationRefund(ctx, row.ID); err != nil {
				return n, err
			}
			continue
		}
		n++
	}
	return n, nil
}
func (s *Server) expireConsultations(ctx context.Context) (int, error) {
	ids, err := s.q.ListConsultationsToExpire(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, id := range ids {
		initial, err := s.q.GetConsultation(ctx, id)
		if err != nil {
			return n, err
		}
		closed := false
		err = s.tx(ctx, func(q *db.Queries) error {
			root, _ := consultationParent(initial)
			if err := q.LockConsultationRoot(ctx, root); err != nil {
				return err
			}
			// 再検索して回答との競合時に古い期限判定を使わない。
			c, err := q.LockConsultation(ctx, id)
			if err != nil {
				return err
			}
			eligible, err := q.ListConsultationsToExpire(ctx)
			if err != nil {
				return err
			}
			found := false
			for _, candidate := range eligible {
				if candidate == id {
					found = true
					break
				}
			}
			if !found {
				return nil
			}
			status := "expired"
			if c.Status == "draft" {
				status = "cancelled"
			}
			c, err = q.CloseConsultation(ctx, db.CloseConsultationParams{ID: id, Status: status})
			if err != nil {
				return err
			}
			if err := s.releaseConsultationFunding(ctx, q, c); err != nil {
				return err
			}
			closed = true
			return nil
		})
		if err != nil {
			return n, err
		}
		if closed {
			n++
			s.notifyConsultation(ctx, initial, false, "追加相談の期限が過ぎました。未回答分は返却・返金します")
		}
	}
	return n, nil
}
