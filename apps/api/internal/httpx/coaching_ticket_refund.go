package httpx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

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
		if l.ExpiresAt != nil && !l.ExpiresAt.After(time.Now()) {
			// 有効期限を過ぎた分は失効させる（返金の対象にしない）。tick の失効処理を待たずに断る。
			return &ticketRuleError{"tickets_expired", "有効期限を過ぎたチケットは返金できません"}
		}
		if l.Available == 0 {
			return &ticketRuleError{"no_refundable_tickets", "未使用・未予約のチケットがありません"}
		}
		n := l.Available
		// 購入金額 ÷ 枚数 × 未使用枚数。1円未満の端数は四捨五入する（本体 #572 v3）。
		// 端数は累積枚数から算出し、複数回に分けても購入額を超えない。
		share := func(cumulative int64) int64 {
			return (2*int64(p.AmountYen)*cumulative + int64(p.Quantity)) / (2 * int64(p.Quantity))
		}
		amount := int32(share(int64(l.Refunded+n)) - share(int64(l.Refunded)))
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

// preparePostExpiryTicketRefund は、失効後の返金（本体 #572 項目7）を用意する。
// 運営者またはコーチの都合で有効期限内に添削を受けられなかったことが確認できた失効済みチケットを、
// 申出ベースで返金する（1枚あたり単発価格500円に相当する額。自動判定はしない）。
// tick の失効処理を待たず、期限を過ぎた未使用分はここで失効に回す。
// 失効から返金への移動は台帳に1枚ずつ記録する（行も台帳も消さない）。
// 確保中（返却待ち）の分は触らず、返却・終了のときに失効へ回る（settleReviewTickets）。
func (s *Server) preparePostExpiryTicketRefund(ctx context.Context, id uuid.UUID, quantity int32) (db.CoachingTicketRefund, error) {
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
		if l.ExpiresAt == nil || l.ExpiresAt.After(time.Now()) {
			// 有効期限の前は通常の返金を使う。失効後の救済は期限を過ぎてから。
			return &ticketRuleError{"tickets_not_expired", "有効期限内のチケットは通常の返金を使ってください"}
		}
		expired := l.Expired
		if l.Available > 0 {
			expired += l.Available
			if err := moveTicketsExpiring(ctx, q, id, fmt.Sprintf("expire-%s-%d", l.ID, expired), "expire", -l.Available, 0, 0, 0, l.Available); err != nil {
				return err
			}
		}
		if quantity < 1 || quantity > expired {
			return &ticketRuleError{"invalid_refund_quantity", "返金できる枚数を確認してください"}
		}
		out, err = q.CreateTicketRefund(ctx, db.CreateTicketRefundParams{ID: uuid.New(), PurchaseID: id, Quantity: quantity, AmountYen: reviewPriceYen * quantity})
		if err != nil {
			return err
		}
		// 失効した枚数から返金に回す。台帳には1枚ずつ記録する。
		for i := int32(0); i < quantity; i++ {
			if err := moveTicketsExpiring(ctx, q, id, fmt.Sprintf("post-expiry-refund-%s-%d", out.ID, i), "refund", 0, 0, 0, 1, -1); err != nil {
				return err
			}
		}
		return nil
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
	JSON(w, 200, map[string]any{"purchases": purchases, "balance_issues": issues, "pending_refunds": refunds, "single_price_yen": reviewPriceYen})
}

func (s *Server) adminRefundTickets(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	// 送るものがないときは通常の返金。失効後の返金は {post_expiry: true, quantity: n} を送る（本体 #572）。
	var req struct {
		PostExpiry bool  `json:"post_expiry"`
		Quantity   int32 `json:"quantity"`
	}
	if err := DecodeJSON(r, &req); err != nil && !errors.Is(err, io.EOF) {
		FieldError(w, map[string]string{"post_expiry": "送り直してください"})
		return
	}
	if req.PostExpiry {
		refund, err := s.preparePostExpiryTicketRefund(r.Context(), id, req.Quantity)
		if err != nil {
			writeCheckoutError(w, err)
			return
		}
		// 失敗しても確保済みの返金記録は残し、tickが照合・再実行する。
		if err := s.processTicketRefund(r.Context(), refund.ID); err != nil {
			slog.Error("ticket post-expiry refund", "refund", refund.ID, "err", err)
		}
		JSON(w, http.StatusAccepted, map[string]any{"refund_id": refund.ID,
			"message": "失効後の返金を受け付けました。処理状況は購入履歴で確認できます"})
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
