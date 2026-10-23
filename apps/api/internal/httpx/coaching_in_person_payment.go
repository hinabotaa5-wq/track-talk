package httpx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
)

func inPersonRemaining(ct db.CoachingContract, usage db.InPersonUsageRow) int32 {
	total := int64(ct.SessionCount)
	if total <= 0 {
		return 0
	}
	return int32(int64(ct.PriceYen) * max(total-int64(usage.Completed), 0) / total)
}

// 未実施分は実施確認済みの回数だけで計算し、期間終了48時間後に精算する。
func (s *Server) prepareInPersonSettlement(ctx context.Context, id uuid.UUID, kind string) error {
	return s.tx(ctx, func(q *db.Queries) error {
		if err := q.LockCoachPayouts(ctx); err != nil {
			return err
		}
		ct, err := q.LockConsultationContract(ctx, id)
		if err != nil {
			return err
		}
		if ct.FulfillmentVersion != 2 {
			return &ticketRuleError{"invalid_in_person", "対面指導の契約を選んでください"}
		}
		if ct.FulfillmentState == "refund_pending" {
			return nil
		}
		if kind == "auto" {
			if ct.FulfillmentState != "open" {
				return nil
			}
			eligible, err := q.ListInPersonSettlements(ctx)
			if err != nil {
				return err
			}
			found := false
			for _, x := range eligible {
				if x == id {
					found = true
					break
				}
			}
			if !found {
				return nil
			}
		}
		earning, err := q.GetContractEarning(ctx, &ct.ID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil && earning.PayoutID != nil {
			return &ticketRuleError{"payout_recorded", "送金に含めた報酬があります。運営で個別に確認してください"}
		}
		pay, err := q.GetPaidCoachingPayment(ctx, &ct.ID)
		if errors.Is(err, pgx.ErrNoRows) && kind == "auto" {
			return q.MarkInPersonSettled(ctx, ct.ID)
		}
		if err != nil {
			return err
		}
		usage, err := q.InPersonUsage(ctx, ct.ID)
		if err != nil {
			return err
		}
		amount := inPersonRemaining(ct, usage)
		reason := "in_person_unprovided"
		if kind == "full" {
			amount = pay.AmountYen
			reason = "in_person_admin_full"
		}
		if ct.FulfillmentState == "refund_failed" && ct.SupportRefundAmountYen != nil {
			amount = *ct.SupportRefundAmountYen
			reason = *ct.SupportRefundReason
		}
		if amount <= 0 {
			if kind != "auto" {
				return &ticketRuleError{"nothing_to_refund", "すべて提供済みのため未提供分はありません"}
			}
			return q.MarkInPersonSettled(ctx, ct.ID)
		}
		operation := uuid.New()
		if _, err := q.PrepareSupportRefund(ctx, db.PrepareSupportRefundParams{ID: ct.ID, SupportRefundAmountYen: &amount, SupportRefundAttemptID: &operation, SupportRefundReason: &reason}); err != nil {
			return err
		}
		if err := q.CancelInPersonSessions(ctx, ct.ID); err != nil {
			return err
		}
		return q.DeleteUnpaidContractEarning(ctx, &ct.ID)
	})
}
func (s *Server) processInPersonRefund(ctx context.Context, id uuid.UUID) error {
	if !s.stripe.Configured() {
		return nil
	}
	return s.tx(ctx, func(q *db.Queries) error {
		if err := q.LockCoachPayouts(ctx); err != nil {
			return err
		}
		ct, err := q.LockConsultationContract(ctx, id)
		if err != nil {
			return err
		}
		if ct.FulfillmentState != "refund_pending" || ct.FulfillmentVersion != 2 {
			return nil
		}
		if ct.SupportRefundAttemptID == nil || ct.SupportRefundAmountYen == nil || ct.SupportRefundReason == nil {
			return errors.New("in-person refund missing operation")
		}
		pay, err := q.GetPaidCoachingPayment(ctx, &id)
		if err != nil {
			return err
		}
		pay, err = q.LockPaidCoachingPayment(ctx, pay.ID)
		if err != nil {
			return err
		}
		if pay.PaymentIntentID == nil {
			return errors.New("in-person payment missing intent")
		}
		operation := "in-person-" + ct.SupportRefundAttemptID.String()
		remote, err := s.stripe.FindTicketRefund(ctx, *pay.PaymentIntentID, operation)
		if err != nil {
			return fmt.Errorf("%w: %w", errStripe, err)
		}
		if remote == nil {
			created, err := s.stripe.CreateRefundWithMetadata(ctx, *pay.PaymentIntentID, int(*ct.SupportRefundAmountYen), operation)
			if err != nil {
				return fmt.Errorf("%w: %w", errStripe, err)
			}
			remote = &created
		}
		if remote.ID == "" || remote.Amount != int(*ct.SupportRefundAmountYen) {
			return errors.New("in-person refund amount mismatch")
		}
		switch remote.Status {
		case "succeeded":
			if _, err := q.RecordCoachingRefund(ctx, db.RecordCoachingRefundParams{ID: pay.ID, RefundedYen: *ct.SupportRefundAmountYen, RefundReason: *ct.SupportRefundReason}); err != nil {
				return err
			}
			if err := q.DeleteUnpaidContractEarning(ctx, &id); err != nil {
				return err
			}
			return q.MarkInPersonSettled(ctx, id)
		case "failed", "canceled":
			return q.FailSupportRefund(ctx, id)
		}
		return nil
	})
}
func (s *Server) settleInPersonContracts(ctx context.Context) (int, error) {
	ids, err := s.q.ListInPersonSettlements(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, id := range ids {
		if err := s.prepareInPersonSettlement(ctx, id, "auto"); err != nil {
			slog.Error("prepare in-person settlement", "contract", id, "err", err)
			continue
		}
		if err := s.processInPersonRefund(ctx, id); err != nil {
			slog.Error("process in-person refund", "contract", id, "err", err)
			continue
		}
		n++
	}
	return n, nil
}
