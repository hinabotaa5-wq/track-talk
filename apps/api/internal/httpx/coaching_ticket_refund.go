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

// 未使用・未予約の枚数だけを返金に回す。先に利用可能残高から除き、外部への返金が終わるまで再利用させない。
func (s *Server) prepareTicketRefund(ctx context.Context, id uuid.UUID) (db.CoachingTicketRefund, error) {
	var out db.CoachingTicketRefund
	err := s.tx(ctx, func(q *db.Queries) error {
		p, err := q.LockTicketPurchase(ctx, id)
		if err != nil {
			return err
		}
		if p.Status == "pending" {
			return &ticketRuleError{"not_paid", "まだ支払いが確定していません"}
		}
		existing, err := q.GetPendingTicketRefund(ctx, id)
		if err == nil {
			out = existing
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		l, err := q.LockTicketLot(ctx, id)
		if err != nil {
			return err
		}
		if l.Available == 0 {
			return &ticketRuleError{"no_refundable_tickets", "未使用・未予約のチケットがありません"}
		}
		n := l.Available
		// 端数は累積枚数から算出し、複数回に分けても購入額を超えない。
		amount := int32(int64(p.AmountYen)*int64(l.Refunded+n)/int64(p.Quantity) - int64(p.AmountYen)*int64(l.Refunded)/int64(p.Quantity))
		if amount <= 0 {
			return &ticketRuleError{"refund_amount_zero", "返金額を確認してください"}
		}
		out, err = q.CreateTicketRefund(ctx, db.CreateTicketRefundParams{ID: uuid.New(), PurchaseID: id, Quantity: n, AmountYen: amount})
		if err != nil {
			return err
		}
		return moveTickets(ctx, q, id, "refund-"+out.ID.String(), "refund", -n, 0, 0, n)
	})
	return out, err
}

func (s *Server) processTicketRefund(ctx context.Context, id uuid.UUID) error {
	return s.tx(ctx, func(q *db.Queries) error {
		refund, err := q.LockTicketRefund(ctx, id)
		if err != nil {
			return err
		}
		if refund.Status != "pending" {
			return nil
		}
		p, err := q.LockTicketPurchase(ctx, refund.PurchaseID)
		if err != nil {
			return err
		}
		if p.PaymentIntentID == nil {
			return errors.New("ticket purchase missing payment intent")
		}
		remote, err := s.stripe.FindTicketRefund(ctx, *p.PaymentIntentID, id.String())
		if err != nil {
			return err
		}
		if remote == nil {
			created, err := s.stripe.CreateRefundWithMetadata(ctx, *p.PaymentIntentID, int(refund.AmountYen), id.String())
			if err != nil {
				return err
			}
			remote = &created
		}
		if remote.ID == "" || remote.Amount != int(refund.AmountYen) {
			return fmt.Errorf("ticket refund amount mismatch")
		}
		status := "pending"
		switch remote.Status {
		case "succeeded":
			status = "succeeded"
			if err := q.FinishTicketPurchaseRefund(ctx, db.FinishTicketPurchaseRefundParams{ID: p.ID, AmountYen: refund.AmountYen}); err != nil {
				return err
			}
		case "failed", "canceled":
			status = "failed"
			if err := moveTickets(ctx, q, p.ID, "refund-failed-"+id.String(), "refund_failed", refund.Quantity, 0, 0, -refund.Quantity); err != nil {
				return err
			}
		}
		return q.SaveTicketRefund(ctx, db.SaveTicketRefundParams{ID: id, StripeRefundID: &remote.ID, Status: status})
	})
}

func (s *Server) settleTicketRefunds(ctx context.Context) (int, error) {
	if !s.stripe.Configured() {
		return 0, nil
	}
	rows, err := s.q.ListPendingTicketRefunds(ctx)
	if err != nil {
		return 0, err
	}
	completed := 0
	for _, r := range rows {
		if err := s.processTicketRefund(ctx, r.ID); err != nil {
			slog.Error("settle ticket refund", "refund", r.ID, "err", err)
			if err := s.q.TouchTicketRefund(ctx, r.ID); err != nil {
				return completed, err
			}
			continue
		}
		completed++
	}
	return completed, nil
}

func (s *Server) adminTickets(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	purchases, err := s.q.AdminTicketPurchases(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	issues, err := s.q.TicketReconciliation(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	refunds, err := s.q.ListPendingTicketRefunds(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, 200, map[string]any{"purchases": purchases, "balance_issues": issues, "pending_refunds": refunds})
}

func (s *Server) adminRefundTickets(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	refund, err := s.prepareTicketRefund(r.Context(), id)
	if err != nil {
		writeCheckoutError(w, err)
		return
	}
	// 失敗しても確保済みの返金記録は残し、tickが照合・再実行する。
	if err := s.processTicketRefund(r.Context(), refund.ID); err != nil {
		slog.Error("ticket refund", "refund", refund.ID, "err", err)
	}
	JSON(w, http.StatusAccepted, map[string]any{"refund_id": refund.ID, "message": "返金を受け付けました。処理状況は購入履歴で確認できます"})
}
