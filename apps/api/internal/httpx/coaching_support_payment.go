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
	"github.com/jackc/pgx/v5"
)

// supportUnlimited は、動画への返答に回数の上限がない契約（無料相談の提案から成立した大会までプラン）。
// 契約の動画回数は 0（回数を決めない）で保存する。
func supportUnlimited(ct db.CoachingContract) bool {
	return ct.VideoReviews != nil && *ct.VideoReviews == 0
}

// supportRemainingByDays は、動画が無制限の契約の未提供分。契約価格 ÷ 期間日数 × 未経過日数（円未満は切り捨て）。
// 期間日数は、開始日〜目標大会の日（両端を含む・日本時間の日付）。開始日は実際に始まった日で、まだ始まっていなければ全額。
// 返金を決める日（今日）は経過した日に数える。通話や動画の回数は数えない（返金の単位は期間で一本化）。
func supportRemainingByDays(ct db.CoachingContract, now time.Time) int32 {
	return supportUnprovidedAsOf(ct, supportDay(now))
}

func supportRemaining(ct db.CoachingContract, usage db.SupportUsageRow) int32 {
	if supportUnlimited(ct) {
		// 途中で終えた契約は終了の日（大会の中止・延期は主催者の発表の日）、それ以外は今日を経過扱いにする。
		return supportUnprovidedAsOf(ct, supportAsOf(ct, time.Now()))
	}
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
		if kind == "end" {
			// 運営が途中で終えた大会までプラン。解約料は取らず、未経過分を日割りで全額返金する。
			// コーチ都合だけは、経過分も含めて全額を返金する。
			if ct.SupportEndCause == nil {
				return &ticketRuleError{"invalid_support", "終わり方が記録されていません"}
			}
			reason = "support_" + *ct.SupportEndCause
			if *ct.SupportEndCause == supportEndCoachFault {
				amount = pay.AmountYen
			}
			if amount <= 0 {
				if _, err := q.EndSupportWithoutRefund(ctx, ct.ID); err != nil {
					return err
				}
				return q.MarkSupportSettled(ctx, ct.ID)
			}
		}
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
