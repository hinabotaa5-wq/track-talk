package httpx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
)

func supportRemaining(ct db.CoachingContract, usage db.SupportUsageRow) int32 {
	total := int64(ct.CallCount)
	if ct.VideoReviews != nil {
		total += int64(*ct.VideoReviews)
	}
	if total <= 0 {
		return 0
	}
	delivered := int64(usage.VideoAnswered + usage.CallsCompleted)
	return int32(int64(ct.PriceYen) * max(total-delivered, 0) / total)
}

// 未提供分は動画回答1組・実施確認済み通話1回を同じ1単位として計算する。
// 返金額と操作IDを先に確定し、外部返金後に落ちても同額・同一操作で再開する。
func (s *Server) prepareSupportSettlement(ctx context.Context, id uuid.UUID, kind string) error {
	return s.tx(ctx, func(q *db.Queries) error {
		if err := q.LockCoachPayouts(ctx); err != nil {
			return err
		}
		ct, err := q.LockConsultationContract(ctx, id)
		if err != nil {
			return err
		}
		if ct.FulfillmentVersion != 1 {
			return &ticketRuleError{"invalid_support", "大会サポートの契約を選んでください"}
		}
		if ct.FulfillmentState == "refund_pending" {
			return nil
		}
		if kind == "auto" {
			if ct.FulfillmentState != "open" {
				return nil
			}
			eligible, err := q.ListSupportSettlements(ctx)
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
			return q.MarkSupportSettled(ctx, ct.ID)
		}
		if err != nil {
			return err
		}
		usage, err := q.SupportUsage(ctx, ct.ID)
		if err != nil {
			return err
		}
		amount := supportRemaining(ct, usage)
		reason := "support_unprovided"
		if kind == "full" {
			amount = pay.AmountYen
			reason = "support_admin_full"
		}
		if kind == "auto" && ct.Status == "active" {
			amount = pay.AmountYen
			reason = "support_no_response"
		}
		if ct.FulfillmentState == "refund_failed" && ct.SupportRefundAmountYen != nil {
			amount = *ct.SupportRefundAmountYen
			reason = *ct.SupportRefundReason
		}
		if amount <= 0 {
			if kind != "auto" {
				return &ticketRuleError{"nothing_to_refund", "すべて提供済みのため未提供分はありません"}
			}
			return q.MarkSupportSettled(ctx, ct.ID)
		}
		operation := uuid.New()
		if _, err := q.PrepareSupportRefund(ctx, db.PrepareSupportRefundParams{ID: ct.ID, SupportRefundAmountYen: &amount, SupportRefundAttemptID: &operation, SupportRefundReason: &reason}); err != nil {
			return err
		}
		if err := q.CancelSupportCalls(ctx, ct.ID); err != nil {
			return err
		}
		return q.DeleteUnpaidContractEarning(ctx, &ct.ID)
	})
}
func (s *Server) processSupportRefund(ctx context.Context, id uuid.UUID) error {
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
		if ct.FulfillmentState != "refund_pending" {
			return nil
		}
		if ct.SupportRefundAttemptID == nil || ct.SupportRefundAmountYen == nil || ct.SupportRefundReason == nil {
			return errors.New("support refund missing operation")
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
			return errors.New("support payment missing intent")
		}
		operation := "support-" + ct.SupportRefundAttemptID.String()
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
			return errors.New("support refund amount mismatch")
		}
		switch remote.Status {
		case "succeeded":
			if _, err := q.RecordCoachingRefund(ctx, db.RecordCoachingRefundParams{ID: pay.ID, RefundedYen: *ct.SupportRefundAmountYen, RefundReason: *ct.SupportRefundReason}); err != nil {
				return err
			}
			if err := q.DeleteUnpaidContractEarning(ctx, &id); err != nil {
				return err
			}
			return q.MarkSupportSettled(ctx, id)
		case "failed", "canceled":
			return q.FailSupportRefund(ctx, id)
		}
		return nil
	})
}
func (s *Server) settleSupportContracts(ctx context.Context) (int, error) {
	ids, err := s.q.ListSupportSettlements(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, id := range ids {
		if err := s.prepareSupportSettlement(ctx, id, "auto"); err != nil {
			slog.Error("prepare support settlement", "contract", id, "err", err)
			continue
		}
		if err := s.processSupportRefund(ctx, id); err != nil {
			slog.Error("process support refund", "contract", id, "err", err)
			continue
		}
		n++
	}
	return n, nil
}
func (s *Server) adminRefundSupport(w http.ResponseWriter, r *http.Request, id uuid.UUID, kind string) {
	if err := s.prepareSupportSettlement(r.Context(), id, kind); err != nil {
		writeCheckoutError(w, err)
		return
	}
	if err := s.processSupportRefund(r.Context(), id); err != nil {
		slog.Error("support refund", "contract", id, "err", err)
	}
	ct, err := s.q.GetCoachingContract(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	out := s.contractJSON(ct)
	if err := s.attachPayment(r.Context(), out, id); err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, 200, map[string]any{"contract": out, "payment": out["payment"]})
}
