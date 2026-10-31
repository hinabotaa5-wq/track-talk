package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/stripe"
	"github.com/jackc/pgx/v5"
)

// Stripe の見覚えのないお金の動きの検知。アプリが出す返金・送金は DB に記録がある。
// 記録と合わない返金・送金が Webhook か日次の照合で見つかったら、運営へメールする。
// メール本文には種類・Stripe のオブジェクト ID・金額・日時だけを入れ、鍵や個人情報は入れない。

type refundNotice struct {
	RefundID      string
	PaymentIntent string
	Amount        int
}

type transferNotice struct {
	ID          string
	Amount      int
	Destination string
	Metadata    map[string]string
}

// parseRefundObject は、refund.created（Refund）と charge.refunded（Charge）の両方を読む。
func parseRefundObject(raw json.RawMessage) (refundNotice, error) {
	var o struct {
		ID             string `json:"id"`
		Amount         int    `json:"amount"`
		AmountRefunded int    `json:"amount_refunded"`
		PaymentIntent  string `json:"payment_intent"`
		Refunds        struct {
			Data []struct {
				ID     string `json:"id"`
				Amount int    `json:"amount"`
			} `json:"data"`
		} `json:"refunds"`
	}
	if err := json.Unmarshal(raw, &o); err != nil {
		return refundNotice{}, err
	}
	n := refundNotice{PaymentIntent: o.PaymentIntent, Amount: o.Amount}
	// Refund オブジェクトは id が re_ で始まる。Charge のときは累計額と先頭の返金を使う。
	if len(o.ID) > 3 && o.ID[:3] == "re_" {
		n.RefundID = o.ID
	} else {
		n.Amount = o.AmountRefunded
		if len(o.Refunds.Data) > 0 {
			n.RefundID = o.Refunds.Data[0].ID
			n.Amount = o.Refunds.Data[0].Amount
		}
	}
	return n, nil
}

func parseTransferObject(raw json.RawMessage) (transferNotice, error) {
	var o transferNotice
	err := json.Unmarshal(raw, &o)
	return o, err
}

// knownRefund は、アプリが出した返金の記録が DB にあるか。
// 返金 ID が残るものは ID で、残らないものは支払い（payment_intent）に返金の記録があるかで確かめる。
func (s *Server) knownRefund(ctx context.Context, n refundNotice) (bool, error) {
	var byID, byPayment bool
	err := s.pool.QueryRow(ctx, `
		SELECT
		  EXISTS(SELECT 1 FROM coaching_ticket_refunds WHERE stripe_refund_id = $1 AND $1 <> '')
		  OR EXISTS(SELECT 1 FROM coaching_consultation_refunds WHERE stripe_refund_id = $1 AND $1 <> '')
 OR EXISTS(SELECT 1 FROM captured_payment_refunds WHERE stripe_refund_id=$1 AND $1<>'' AND origin='admin'),
		  EXISTS(SELECT 1 FROM coaching_payments WHERE payment_intent_id = $2 AND $2 <> '' AND refunded_yen > 0)
		  OR EXISTS(SELECT 1 FROM coaching_ticket_purchases WHERE payment_intent_id = $2 AND $2 <> '' AND refunded_yen > 0)
		  OR EXISTS(SELECT 1 FROM review_payments WHERE payment_intent_id = $2 AND $2 <> '' AND refunded_at IS NOT NULL)`,
		n.RefundID, n.PaymentIntent).Scan(&byID, &byPayment)
	return refundKnown(byID, byPayment), err
}

// payoutRecord は、送金の照合に使う DB の記録の抜き出し。
type payoutRecord struct {
	TransferID      string
	AmountYen       int
	StripeAccountID string
}

// transferKnown は、送金が DB の記録と合うかの判定そのもの。
// 送金 ID の一致か、metadata の payout_id で引いた記録の金額・送り先口座の一致で確かめる。
func transferKnown(byTransferID, byPayoutID *payoutRecord, n transferNotice) bool {
	if byTransferID != nil {
		return true
	}
	if byPayoutID == nil {
		return false
	}
	return byPayoutID.AmountYen == n.Amount && byPayoutID.StripeAccountID == n.Destination
}

// knownTransfer は、runCoachPayouts が作った送金の記録と合うか。
func (s *Server) knownTransfer(ctx context.Context, n transferNotice) (bool, error) {
	find := func(query string, arg any) (*payoutRecord, error) {
		var row payoutRecord
		err := s.pool.QueryRow(ctx, query, arg).Scan(&row.TransferID, &row.AmountYen, &row.StripeAccountID)
		if errors.Is(err, pgx.ErrNoRows) {
			// 記録が無いのは異常ではなく「見覚えがない」側。
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return &row, nil
	}
	byTransferID, err := find(`
		SELECT COALESCE(p.stripe_transfer_id, ''), p.amount_yen, COALESCE(cp.stripe_account_id, '')
		FROM coach_payouts p LEFT JOIN coach_profiles cp ON cp.profile_id = p.coach_id
		WHERE p.stripe_transfer_id = $1 AND $1 <> ''`, n.ID)
	if err != nil {
		return false, err
	}
	var byPayoutID *payoutRecord
	if payoutID := n.Metadata["payout_id"]; payoutID != "" {
		if _, err := uuid.Parse(payoutID); err == nil {
			byPayoutID, err = find(`
				SELECT COALESCE(p.stripe_transfer_id, ''), p.amount_yen, COALESCE(cp.stripe_account_id, '')
				FROM coach_payouts p LEFT JOIN coach_profiles cp ON cp.profile_id = p.coach_id
				WHERE p.id::text = $1`, payoutID)
			if err != nil {
				return false, err
			}
		}
	}
	return transferKnown(byTransferID, byPayoutID, n), nil
}

// refundKnown は、返金の照合の判定そのもの。返金 ID の一致か、
// 支払い（payment_intent）にアプリの返金の記録があるか。
func refundKnown(refundIDMatched, paymentIntentRefunded bool) bool {
	return refundIDMatched || paymentIntentRefunded
}

// alertUnknownRefund は、記録のない返金だけを運営へ知らせる。同じ返金は1回だけ。
func (s *Server) alertUnknownRefund(ctx context.Context, n refundNotice, when time.Time) {
	known, err := s.knownRefund(ctx, n)
	if err != nil {
		slog.Warn("stripe refund check", "refund", n.RefundID, "err", err)
		return
	}
	if known {
		return
	}
	key := "refund:" + n.RefundID
	if n.RefundID == "" {
		key = "refund-pi:" + n.PaymentIntent
	}
	if !s.markSecurityConnectionSeen(key) {
		return
	}
	slog.Warn("unknown stripe refund", "refund", n.RefundID, "payment_intent", n.PaymentIntent, "amount", n.Amount)
	s.alertOps(ctx, "見覚えのない返金がありました",
		fmt.Sprintf("種類: 返金\nStripe ID: %s\n支払い: %s\n金額: %d円\n日時: %s\nダッシュボード: https://dashboard.stripe.com/payments/%s\n\n心当たりがなければ、Stripe のセキュリティ履歴を確認し、APIキーを入れ替えてください。",
			n.RefundID, n.PaymentIntent, n.Amount, when.Format(time.RFC3339), n.PaymentIntent),
		key)
}

// alertUnknownTransfer は、記録のない送金だけを運営へ知らせる。同じ送金は1回だけ。
func (s *Server) alertUnknownTransfer(ctx context.Context, n transferNotice, when time.Time) {
	known, err := s.knownTransfer(ctx, n)
	if err != nil {
		slog.Warn("stripe transfer check", "transfer", n.ID, "err", err)
		return
	}
	if known {
		return
	}
	key := "transfer:" + n.ID
	if !s.markSecurityConnectionSeen(key) {
		return
	}
	slog.Warn("unknown stripe transfer", "transfer", n.ID, "amount", n.Amount, "destination", n.Destination)
	s.alertOps(ctx, "見覚えのない送金がありました",
		fmt.Sprintf("種類: 送金\nStripe ID: %s\n金額: %d円\n日時: %s\nダッシュボード: https://dashboard.stripe.com/connect/transfers/%s\n\n心当たりがなければ、Stripe のセキュリティ履歴を確認し、APIキーを入れ替えてください。",
			n.ID, n.Amount, when.Format(time.RFC3339), n.ID),
		key)
}

// handleStripeMoneyAlert は、返金・送金の Webhook を照合する。読めない知らせでも落とさず、記録だけ残す。
func (s *Server) handleStripeMoneyAlert(ctx context.Context, ev stripe.Event) {
	switch ev.Type {
	case "charge.refunded", "refund.created":
		n, err := parseRefundObject(ev.Data.Object)
		if err != nil {
			slog.Warn("stripe refund parse", "event", ev.ID, "err", err)
			return
		}
		s.alertUnknownRefund(ctx, n, s.now())
	case "transfer.created":
		n, err := parseTransferObject(ev.Data.Object)
		if err != nil {
			slog.Warn("stripe transfer parse", "event", ev.ID, "err", err)
			return
		}
		s.alertUnknownTransfer(ctx, n, s.now())
	case "transfer.reversed":
		n, err := parseTransferObject(ev.Data.Object)
		if err != nil {
			slog.Warn("stripe transfer parse", "event", ev.ID, "err", err)
			return
		}
		// アプリは送金を取り消さない。取り消し自体を見覚えのない動きとして知らせる。
		key := "transfer-reversed:" + n.ID
		if s.markSecurityConnectionSeen(key) {
			slog.Warn("stripe transfer reversed", "transfer", n.ID, "amount", n.Amount)
			s.alertOps(ctx, "送金が取り消されました",
				fmt.Sprintf("種類: 送金の取り消し\nStripe ID: %s\n金額: %d円\n日時: %s\nダッシュボード: https://dashboard.stripe.com/connect/transfers/%s",
					n.ID, n.Amount, s.now().Format(time.RFC3339), n.ID), key)
		}
	case "payout.created":
		var p struct {
			ID     string `json:"id"`
			Amount int    `json:"amount"`
		}
		_ = json.Unmarshal(ev.Data.Object, &p)
		s.checkPayout(ctx, p.ID, p.Amount, s.now())
	}
}

// payoutLast4Mismatch は、入金の宛先チェックの判定。期待値が未設定か末尾4桁が読めなければ不一致とは言わない（安全側）。
func payoutLast4Mismatch(expected, actual string) bool {
	return expected != "" && actual != "" && expected != actual
}

// checkPayout は、運営の銀行への入金の宛先がいつもの口座か確かめる。
// OPS_PAYOUT_EXPECTED_LAST4 が未設定なら記録だけ（Stripe へも聞かない）。
// 設定時は宛先の末尾4桁だけを Stripe から読み、違えば運営へ知らせる。
// メール本文には末尾4桁も含めず、種類・ID・金額・日時・ダッシュボード URL だけを入れる。
func (s *Server) checkPayout(ctx context.Context, payoutID string, amount int, when time.Time) bool {
	slog.Info("stripe payout", "payout", payoutID, "amount", amount)
	expected := s.cfg.OpsPayoutExpectedLast4
	if expected == "" {
		return false
	}
	actual, err := s.stripe.GetPayoutDestinationLast4(ctx, payoutID)
	if err != nil || actual == "" {
		// 宛先が読めなければ、不一致と決めつけず記録だけ残す。
		slog.Warn("stripe payout destination", "payout", payoutID, "err", err)
		return false
	}
	if !payoutLast4Mismatch(expected, actual) {
		return false
	}
	key := "payout:" + payoutID
	if !s.markSecurityConnectionSeen(key) {
		return false
	}
	slog.Warn("stripe payout destination mismatch", "payout", payoutID, "amount", amount)
	return s.alertOps(ctx, "入金先の口座がいつもと違います",
		fmt.Sprintf("種類: 銀行への入金\nStripe ID: %s\n金額: %d円\n日時: %s\nダッシュボード: https://dashboard.stripe.com/payouts/%s\n\n心当たりがなければ、Stripe のセキュリティ履歴と入金先口座を確認してください。",
			payoutID, amount, when.Format(time.RFC3339), payoutID),
		key)
}

// reconcileStripeMoney は、Webhook の取りこぼし対策の日次照合。過去25時間の返金・送金を
// Stripe から一覧し、同じ照合にかける。1日に1回だけ（プロセス再起動時はその日に再実行されうる）。
func (s *Server) reconcileStripeMoney(ctx context.Context) error {
	if !s.stripe.Configured() {
		return nil
	}
	day := s.now().Format("2006-01-02")
	s.securityMu.Lock()
	if s.stripeReconciledDay == day {
		s.securityMu.Unlock()
		return nil
	}
	s.stripeReconciledDay = day
	s.securityMu.Unlock()

	since := s.now().Add(-25 * time.Hour)
	refunds, err := s.stripe.ListRefunds(ctx, since)
	if err != nil {
		return err
	}
	for _, r := range refunds {
		s.alertUnknownRefund(ctx, refundNotice{RefundID: r.ID, PaymentIntent: r.PaymentIntent, Amount: r.Amount}, s.now())
	}
	transfers, err := s.stripe.ListTransfers(ctx, since)
	if err != nil {
		return err
	}
	for _, tr := range transfers {
		s.alertUnknownTransfer(ctx, transferNotice{ID: tr.ID, Amount: tr.Amount, Destination: tr.Destination, Metadata: tr.Metadata}, s.now())
	}
	return nil
}
