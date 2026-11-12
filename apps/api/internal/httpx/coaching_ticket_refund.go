package httpx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/stripe"
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
		// 1枚500円の約束は変えない。ただし Stripe は元の決済額を超えて返金できないので、
		// Stripe へはこの決済の返金できる残り（決済額 − Stripe で返し終えた額）までとし、
		// 超えた分は運営者が Stripe の外（銀行振込など）で返す（運営者決定 2026-11-12）。
		requested := reviewPriceYen * quantity
		stripeRefunded, err := q.SumTicketStripeRefunded(ctx, id)
		if err != nil {
			return err
		}
		stripePart, manualPart := splitPostExpiryRefund(requested, p.AmountYen, stripeRefunded)
		out, err = q.CreatePostExpiryTicketRefund(ctx, db.CreatePostExpiryTicketRefundParams{ID: uuid.New(), PurchaseID: id, Quantity: quantity, AmountYen: stripePart + manualPart, ManualYen: manualPart})
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
		// Stripe へ返すのは合計から Stripe の外で返す分を除いた額（本体 #572）。
		stripeAmount := refund.AmountYen - refund.ManualYen
		if stripeAmount <= 0 {
			// 全額を Stripe の外で返す返金。Stripe には何も送らない。
			// 購入の返金済み額には Stripe 外の分も含めて1回だけ足す（000162 の上限の判定に含める）。
			if err := q.FinishTicketPurchaseRefund(ctx, db.FinishTicketPurchaseRefundParams{ID: p.ID, AmountYen: refund.AmountYen}); err != nil {
				return err
			}
			return q.SaveTicketRefund(ctx, db.SaveTicketRefundParams{ID: id, Status: "succeeded"})
		}
		remote, err := s.stripe.FindTicketRefund(ctx, *p.PaymentIntentID, id.String())
		if err != nil {
			return err
		}
		if remote == nil {
			created, err := s.stripe.CreateRefundWithMetadata(ctx, *p.PaymentIntentID, int(stripeAmount), id.String())
			if err != nil {
				if refundAmountRejected(err) {
					// 金額の超過で断られた返金は、送り直しても通らない。再試行をやめて運営者の対応待ちにする。
					slog.Error("ticket refund rejected for amount; needs operator action", "refund", id, "err", err)
					return q.MarkTicketRefundNeedsAction(ctx, id)
				}
				return err
			}
			remote = &created
		}
		if remote.ID == "" || remote.Amount != int(stripeAmount) {
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
			if refund.PostExpiry {
				// 失効後の返金が失敗したら、返金に回した枚数を失効に戻す（使える枚数には戻さない）。
				if err := moveTicketsExpiring(ctx, q, p.ID, "refund-failed-"+id.String(), "refund_failed", 0, 0, 0, -refund.Quantity, refund.Quantity); err != nil {
					return err
				}
			} else if err := moveTickets(ctx, q, p.ID, "refund-failed-"+id.String(), "refund_failed", refund.Quantity, 0, 0, -refund.Quantity); err != nil {
				return err
			}
		}
		return q.SaveTicketRefund(ctx, db.SaveTicketRefundParams{ID: id, StripeRefundID: &remote.ID, Status: status})
	})
}

// splitPostExpiryRefund は、失効後の返金の要求額を、Stripe で返す分と Stripe の外で返す分に分ける。
// Stripe 分は min(要求額, 決済額 − Stripe で返し終えた額)。複数回に分けても累計で同じ計算になる。
func splitPostExpiryRefund(requested, chargeYen, stripeRefunded int32) (stripePart, manualPart int32) {
	room := chargeYen - stripeRefunded
	if room < 0 {
		room = 0
	}
	stripePart = min(requested, room)
	return stripePart, requested - stripePart
}

// refundAmountRejected は、Stripe が返金を金額の超過（元の決済額を超える・返金済み）で断ったかどうか。
// 通信の失敗や 429・5xx などの一時的な失敗は false（従来どおり tick が再試行する）。
func refundAmountRejected(err error) bool {
	var apiErr *stripe.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusBadRequest {
		return false
	}
	switch apiErr.Code {
	case "amount_too_large", "charge_already_refunded":
		return true
	}
	return apiErr.Type == "invalid_request_error" && strings.Contains(apiErr.Message, "greater than")
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
	attention, err := s.q.AdminTicketRefundsNeedingAttention(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, 200, map[string]any{"purchases": purchases, "balance_issues": issues, "pending_refunds": refunds,
		"attention_refunds": attention, "single_price_yen": reviewPriceYen})
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
		// 一時的な失敗なら確保済みの返金記録は残し、tickが照合・再実行する。
		if err := s.processTicketRefund(r.Context(), refund.ID); err != nil {
			slog.Error("ticket post-expiry refund", "refund", refund.ID, "err", err)
		}
		// 受け付けただけの 202 で済ませず、Stripe 分・Stripe 外の分と今の状態を返す。
		current, err := s.q.LockTicketRefund(r.Context(), refund.ID)
		if err != nil {
			MapDBError(w, err)
			return
		}
		writeTicketRefundResult(w, current)
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

// writeTicketRefundResult は、失効後の返金の結果を返す。
// 返し終えた（Stripe 外の分は支払い待ちを含む）なら200、Stripe の結果待ちなら202、運営者の対応待ちなら409。
func writeTicketRefundResult(w http.ResponseWriter, refund db.CoachingTicketRefund) {
	body := map[string]any{"refund_id": refund.ID, "status": refund.Status, "amount_yen": refund.AmountYen,
		"stripe_yen": refund.AmountYen - refund.ManualYen, "manual_yen": refund.ManualYen}
	switch refund.Status {
	case "succeeded":
		body["message"] = "失効後の返金を行いました"
		if refund.ManualYen > 0 {
			body["message"] = fmt.Sprintf("失効後の返金を行いました。うち%d円は Stripe では返せないため、振込などで返してから完了を記録してください", refund.ManualYen)
		}
		JSON(w, http.StatusOK, body)
	case "needs_action":
		body["error"] = "refund_needs_action"
		body["message"] = "Stripe が返金を断りました（金額の超過）。振込などで返してから完了を記録してください"
		JSON(w, http.StatusConflict, body)
	case "failed":
		body["error"] = "refund_failed"
		body["message"] = "返金に失敗しました。チケットは失効に戻しました"
		JSON(w, http.StatusConflict, body)
	default:
		body["message"] = "失効後の返金を受け付けました。Stripe の処理を待っています"
		JSON(w, http.StatusAccepted, body)
	}
}

// adminCompleteManualTicketRefund は、Stripe の外で返した分（振込など）の支払いを運営者が記録する（本体 #572）。
// Stripe 外の分が残っている返金はその分を、Stripe が断った返金（needs_action）は全額を、Stripe 外で返したものとして完了にする。
func (s *Server) adminCompleteManualTicketRefund(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "refundID")
	if !ok {
		return
	}
	var req struct {
		Method string `json:"method"`
		PaidOn string `json:"paid_on"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		FieldError(w, map[string]string{"method": "送り直してください"})
		return
	}
	method := strings.TrimSpace(req.Method)
	if method == "" || len([]rune(method)) > 40 {
		FieldError(w, map[string]string{"method": "支払方法を40文字以内で入れてください"})
		return
	}
	paidOn, err := time.Parse("2006-01-02", req.PaidOn)
	if err != nil || paidOn.After(time.Now().Add(24*time.Hour)) {
		FieldError(w, map[string]string{"paid_on": "支払った日を入れてください"})
		return
	}
	var out db.CoachingTicketRefund
	err = s.tx(r.Context(), func(q *db.Queries) error {
		refund, err := q.LockTicketRefund(r.Context(), id)
		if err != nil {
			return err
		}
		if _, err := q.LockTicketPurchase(r.Context(), refund.PurchaseID); err != nil {
			return err
		}
		date := &paidOn
		switch {
		case refund.Status == "needs_action":
			out, err = q.ConvertTicketRefundToManual(r.Context(), db.ConvertTicketRefundToManualParams{ID: id, Method: &method, PaidOn: date})
			if err != nil {
				return err
			}
			// Stripe で返していないので、ここで初めて購入の返金済み額に足す。
			return q.FinishTicketPurchaseRefund(r.Context(), db.FinishTicketPurchaseRefundParams{ID: refund.PurchaseID, AmountYen: refund.AmountYen})
		case refund.Status == "succeeded" && refund.ManualYen > 0 && refund.ManualCompletedAt == nil:
			// 返金済み額には Stripe 分と一緒に足し終えている。二重に足さない。
			out, err = q.CompleteTicketRefundManual(r.Context(), db.CompleteTicketRefundManualParams{ID: id, Method: &method, PaidOn: date})
			return err
		default:
			return &ticketRuleError{"no_manual_refund", "Stripe の外で返す分が残っていない返金です"}
		}
	})
	if err != nil {
		writeCheckoutError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"refund": out, "message": "Stripe の外で返した分を記録しました"})
}
