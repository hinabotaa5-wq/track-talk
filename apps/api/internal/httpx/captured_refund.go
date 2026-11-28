package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
	"github.com/hinata/track-talk/apps/api/internal/stripe"
	"github.com/jackc/pgx/v5"
)

// This transaction lock is shared with the complete payout run, including Stripe calls.
func (s *Server) refundTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = db.New(tx).LockCoachPayouts(ctx); err != nil {
		return err
	}
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type capturedRefund struct {
	ID        uuid.UUID `json:"id"`
	Source    string    `json:"source"`
	PaymentID uuid.UUID `json:"payment_id"`
	Intent    string    `json:"payment_intent_id"`
	Amount    int       `json:"amount_yen"`
	Status    string    `json:"status"`
	Remote    *string   `json:"stripe_refund_id"`
}

func loadCapturedRefund(ctx context.Context, tx pgx.Tx, id uuid.UUID) (capturedRefund, error) {
	var f capturedRefund
	err := tx.QueryRow(ctx, `SELECT id,source,payment_id,payment_intent_id,amount_yen,status,stripe_refund_id FROM captured_payment_refunds WHERE id=$1 FOR UPDATE`, id).Scan(&f.ID, &f.Source, &f.PaymentID, &f.Intent, &f.Amount, &f.Status, &f.Remote)
	return f, err
}

// Source is an allowlisted discriminator, never interpolated into SQL.
func capturedPayment(ctx context.Context, tx pgx.Tx, source string, id uuid.UUID) (intent, status, method string, amount int, err error) {
	switch source {
	case "review":
		err = tx.QueryRow(ctx, `SELECT COALESCE(payment_intent_id,''),status,method,amount_yen FROM review_payments WHERE id=$1 FOR UPDATE`, id).Scan(&intent, &status, &method, &amount)
	case "consultation":
		err = tx.QueryRow(ctx, `SELECT COALESCE(payment_intent_id,''),payment_state,'card',price_yen FROM coaching_consultations WHERE id=$1 AND funding='cash' FOR UPDATE`, id).Scan(&intent, &status, &method, &amount)
	default:
		err = pgx.ErrNoRows
	}
	return
}

// Pre-migration orphaned payments cannot be safely assigned to coach earnings.
func missingReviewRefundLink(ctx context.Context, tx pgx.Tx, source string, id uuid.UUID) (bool, error) {
	if source != "review" {
		return false, nil
	}
	var missing bool
	err := tx.QueryRow(ctx, `SELECT request_id IS NULL AND NOT EXISTS(SELECT 1 FROM review_payment_earning_links WHERE payment_id=$1) FROM review_payments WHERE id=$1`, id).Scan(&missing)
	return missing, err
}

// capturedRefundTotals は、支払いの返金操作のうち成功した額の合計（返金累計）と、処理中の額の合計。
func capturedRefundTotals(ctx context.Context, tx pgx.Tx, source string, id uuid.UUID) (succeeded, pending int, err error) {
	err = tx.QueryRow(ctx, `SELECT COALESCE(sum(amount_yen) FILTER (WHERE status='succeeded'),0)::int,COALESCE(sum(amount_yen) FILTER (WHERE status='pending'),0)::int FROM captured_payment_refunds WHERE source=$1 AND payment_id=$2`, source, id).Scan(&succeeded, &pending)
	return
}

// settleCapturedPayment は、支払いの状態と返金累計（refunded_yen）を、返金操作の記録から決め直す（本体 #1097）。
// 累計が支払額に達したら refunded、処理中の返金で全額に達するなら refund_pending、一部だけなら partially_refunded、
// 返金が無ければ captured（追加質問は paid）。失敗・取り消し・処理中の返金は累計に数えない。
// 処理中の部分返金だけでは refund_pending にしない（「全額返金を処理中」と見せないため）。処理中の返金がある支払いの報酬は、
// 状態に関わらず送金の相殺の対象から外れる（planCoachRefundOffsets）。
func settleCapturedPayment(ctx context.Context, tx pgx.Tx, source string, id uuid.UUID) (string, error) {
	_, _, _, amount, err := capturedPayment(ctx, tx, source, id)
	if err != nil {
		return "", err
	}
	refunded, pending, err := capturedRefundTotals(ctx, tx, source, id)
	if err != nil {
		return "", err
	}
	status := "captured"
	switch {
	case refunded >= amount:
		status, refunded = "refunded", amount
	case pending > 0 && refunded+pending >= amount:
		status = "refund_pending"
	case refunded > 0:
		status = "partially_refunded"
	}
	if source == "review" {
		_, err = tx.Exec(ctx, `UPDATE review_payments SET status=$2,refunded_yen=$3,refunded_at=CASE WHEN $2='refunded' THEN now() WHEN $2 IN ('captured','partially_refunded') THEN NULL ELSE refunded_at END,updated_at=now() WHERE id=$1`, id, status, refunded)
		return status, err
	}
	if status == "captured" {
		status = "paid"
	}
	if _, err = tx.Exec(ctx, `UPDATE coaching_consultations SET payment_state=$2,refunded_yen=$3,updated_at=now() WHERE id=$1`, id, status, refunded); err != nil {
		return "", err
	}
	if status == "paid" || status == "partially_refunded" {
		return status, db.New(tx).RecordConsultationEarnings(ctx, id)
	}
	return status, nil
}

func setCapturedPayment(ctx context.Context, tx pgx.Tx, f capturedRefund) error {
	_, err := settleCapturedPayment(ctx, tx, f.Source, f.PaymentID)
	return err
}

// rebalanceCapturedRefundEarnings は、部分返金の累計に合わせてコーチ報酬の減額（相殺台帳）を差分だけ足す。
// 丸めは floor(報酬額 × 返金累計 ÷ 支払額)。実体は migration 000201 の DB 関数（報酬を後から記録したときのトリガーと共通）。
func rebalanceCapturedRefundEarnings(ctx context.Context, tx pgx.Tx, source string, id uuid.UUID, anchor *uuid.UUID) error {
	_, err := tx.Exec(ctx, `SELECT rebalance_captured_refund_earnings($1,$2,$3)`, source, id, anchor)
	return err
}
func (s *Server) prepareCapturedRefund(ctx context.Context, source string, id, admin uuid.UUID, reason string) (uuid.UUID, error) {
	var operation uuid.UUID
	err := s.refundTx(ctx, func(tx pgx.Tx) error {
		intent, status, method, amount, err := capturedPayment(ctx, tx, source, id)
		if err != nil {
			return err
		}
		// Reuse the durable operation even after a successful response was lost.
		// 管理画面からの返金は支払いごとに1件。無ければ、処理中の Dashboard 返金を照合し直す（新しく作らない）。
		// 返金済みの支払いは、最後の返金操作を返す（同じ返金を二重に作らない）。
		err = tx.QueryRow(ctx, `SELECT id FROM captured_payment_refunds WHERE source=$1 AND payment_id=$2 AND (
 (origin='admin' AND status<>'failed') OR status='pending' OR ($3='refunded' AND status='succeeded'))
 ORDER BY CASE WHEN origin='admin' THEN 0 WHEN status='pending' THEN 1 ELSE 2 END,created_at DESC,id LIMIT 1`, source, id, status).Scan(&operation)
		if err == nil {
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		missing, err := missingReviewRefundLink(ctx, tx, source, id)
		if err != nil {
			return err
		}
		if missing {
			return &ticketRuleError{"reconciliation_required", "元の報酬記録との対応を確認してから返金してください"}
		}
		if (status != "captured" && status != "partially_refunded" && !(source == "consultation" && status == "paid")) || intent == "" || method != "card" {
			return &ticketRuleError{"not_refundable", "確定済みのカード支払いだけ返金できます"}
		}
		// 一部返金済みなら残額だけを返金する（管理画面から返金するのは常に全額か残額）。
		refunded, _, err := capturedRefundTotals(ctx, tx, source, id)
		if err != nil {
			return err
		}
		if refunded >= amount {
			return &ticketRuleError{"not_refundable", "確定済みのカード支払いだけ返金できます"}
		}
		amount -= refunded
		// Legacy unfulfilled-consultation refunds own their operations separately.
		var legacy bool
		if source == "consultation" {
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM coaching_consultation_refunds WHERE consultation_id=$1 AND status<>'failed')`, id).Scan(&legacy); err != nil {
				return err
			}
			if legacy {
				return &ticketRuleError{"refund_exists", "既存の返金処理を確認してください"}
			}
		}
		operation = uuid.New()
		_, err = tx.Exec(ctx, `INSERT INTO captured_payment_refunds(id,source,payment_id,payment_intent_id,amount_yen,reason,admin_id,origin) VALUES($1,$2,$3,$4,$5,$6,$7,'admin')`, operation, source, id, intent, amount, reason, admin)
		if err != nil {
			return err
		}
		return setCapturedPayment(ctx, tx, capturedRefund{Source: source, PaymentID: id})
	})
	return operation, err
}

// On final success, exclude unsent earnings, preserving historical payouts.
func finalizeCapturedRefund(ctx context.Context, tx pgx.Tx, f capturedRefund, remote stripe.Refund) error {
	if f.Status == "failed" {
		return nil
	}
	if f.Remote != nil && *f.Remote != remote.ID {
		return errors.New("refund identity mismatch")
	}
	if remote.ID == "" || remote.Amount != f.Amount {
		return errors.New("refund amount mismatch")
	}
	status := "pending"
	switch remote.Status {
	case "succeeded":
		status = "succeeded"
	case "failed", "canceled":
		status = "failed"
	}
	if f.Status == "succeeded" && status != "failed" {
		if status == "succeeded" {
			// Recovery confirmed: the same Stripe refund ID and full amount were
			// read back as succeeded. Clear only the transient reconciliation
			// warning so the admin display stops alerting. Bank failure reasons
			// are kept, and success identity, amount, and timestamps are untouched.
			// A later confirmed failure still falls through to failure handling.
			if _, err := tx.Exec(ctx, `UPDATE captured_payment_refunds SET failure=NULL,updated_at=now() WHERE id=$1 AND failure='Stripe reconciliation required'`, f.ID); err != nil {
				return err
			}
		}
		return nil
	}
	if _, err := tx.Exec(ctx, `UPDATE captured_payment_refunds SET status=$2,stripe_refund_id=$3,failure=$4,succeeded_at=CASE WHEN $2='succeeded' THEN COALESCE(succeeded_at,now()) ELSE succeeded_at END,updated_at=now() WHERE id=$1`, f.ID, status, remote.ID, func() any {
		if status == "failed" {
			if remote.FailureReason != "" {
				return "Stripe refund failed: " + remote.FailureReason
			}
			return "Stripe refund " + remote.Status
		}
		return nil
	}()); err != nil {
		return err
	}
	_, _, _, total, err := capturedPayment(ctx, tx, f.Source, f.PaymentID)
	if err != nil {
		return err
	}
	// 1回で全額を返金した操作は従来の全額の確定処理（未送金の除外・送金済みの相殺）。
	// それより少ない返金（Dashboard の部分返金と、その後の残額の返金）は、累計に対する按分で報酬を減らす。
	full := f.Amount >= total
	if status == "failed" {
		if f.Status == "succeeded" {
			if err := db.New(tx).DeleteRefundCompletionNotice(ctx, f.ID); err != nil {
				return err
			}
			if err := compensateFailedRefund(ctx, tx, f); err != nil {
				return err
			}
		}
		if err := setCapturedPayment(ctx, tx, f); err != nil {
			return err
		}
		if f.Status == "succeeded" && !full {
			// 取り消した分を除いた累計で、残る部分返金の減額を目標額に合わせ直す。
			return rebalanceCapturedRefundEarnings(ctx, tx, f.Source, f.PaymentID, nil)
		}
		return nil
	}
	if status == "pending" {
		return setCapturedPayment(ctx, tx, f)
	}
	settled, err := settleCapturedPayment(ctx, tx, f.Source, f.PaymentID)
	if err != nil {
		return err
	}
	if !full {
		if err := rebalanceCapturedRefundEarnings(ctx, tx, f.Source, f.PaymentID, &f.ID); err != nil {
			return err
		}
		return enqueueCapturedRefundNotice(ctx, tx, f, settled != "refunded")
	}
	rows, err := tx.Query(ctx, `SELECT e.id,e.coach_id,e.amount_yen,e.payout_id FROM coach_earnings e WHERE
 ($1='review' AND e.source='review' AND e.id IN (SELECT earning_id FROM review_payment_earning_links WHERE payment_id=$2)) OR
 ($1='consultation' AND e.source='consultation' AND e.consultation_id=$2) FOR UPDATE`, f.Source, f.PaymentID)
	if err != nil {
		return err
	}
	type earning struct {
		id            uuid.UUID
		coach, payout *uuid.UUID
		amount        int
	}
	var es []earning
	for rows.Next() {
		var e earning
		if err := rows.Scan(&e.id, &e.coach, &e.amount, &e.payout); err != nil {
			rows.Close()
			return err
		}
		es = append(es, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, e := range es {
		// A prepared payout may already have succeeded remotely. Never rewrite it.
		if _, err = tx.Exec(ctx, `INSERT INTO captured_refund_earnings(earning_id,refund_id,excluded) VALUES($1,$2,$3) ON CONFLICT(earning_id) DO UPDATE SET refund_id=EXCLUDED.refund_id,excluded=EXCLUDED.excluded`, e.id, f.ID, e.payout == nil); err != nil {
			return err
		}
		if e.payout != nil {
			if _, err = tx.Exec(ctx, `INSERT INTO coach_refund_adjustments(id,refund_id,earning_id,coach_id,amount_yen) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, uuid.New(), f.ID, e.id, e.coach, e.amount); err != nil {
				return err
			}
		} else {
			// If this earning already paid down older debt, restore that debt: the
			// refunded earning can no longer be used as consideration for an offset.
			if _, err = tx.Exec(ctx, `SELECT reverse_coach_refund_applications($1)`, e.id); err != nil {
				return err
			}
		}
	}
	return enqueueCapturedRefundNotice(ctx, tx, f, false)
}

// enqueueCapturedRefundNotice は、支払者への返金の通知を作る。partial なら一部返金の通知（返金額を出す）にし、
// 「返金が完了しました」と全額の完了に見える文面は使わない。累計が全額に達したら従来の返金完了の通知。
func enqueueCapturedRefundNotice(ctx context.Context, tx pgx.Tx, f capturedRefund, partial bool) error {
	q := db.New(tx)
	if f.Source == "review" {
		if partial {
			return q.EnqueueReviewPartialRefundNotice(ctx, db.EnqueueReviewPartialRefundNoticeParams{
				OperationID: f.ID, PaymentID: f.PaymentID, AmountYen: int32(f.Amount),
			})
		}
		return q.EnqueueReviewRefundNotice(ctx, db.EnqueueReviewRefundNoticeParams{
			OperationID: f.ID, PaymentID: f.PaymentID, Kind: "review_refund",
		})
	}
	consultation, err := q.GetConsultation(ctx, f.PaymentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if partial {
		if consultation.RequesterID == nil {
			return nil
		}
		return q.EnqueueConsultationPartialRefundNotice(ctx, db.EnqueueConsultationPartialRefundNoticeParams{
			ID: f.ID, RecipientID: *consultation.RequesterID, Url: consultationNoticePath(consultation, false), AmountYen: int32(f.Amount),
		})
	}
	return enqueueConsultationRefundNotice(ctx, q, f.ID, consultation)
}

func (s *Server) processCapturedRefund(ctx context.Context, id uuid.UUID) error {
	var notify bool
	err := s.refundTx(ctx, func(tx pgx.Tx) error {
		f, err := loadCapturedRefund(ctx, tx, id)
		if err != nil {
			return err
		}
		if f.Status == "failed" {
			return nil
		}
		// Same permanent metadata lookup as ticket refunds; retries after 24h remain safe.
		operation := "captured-" + f.ID.String()
		var remote *stripe.Refund
		if f.Remote != nil {
			remote, err = s.stripe.FindRefundByID(ctx, f.Intent, *f.Remote)
		} else {
			remote, err = s.stripe.FindTicketRefund(ctx, f.Intent, operation)
		}
		if err != nil {
			return err
		}
		if remote == nil && f.Remote != nil {
			return errors.New("known refund missing from Stripe; reconciliation required")
		}
		if remote == nil && f.Status == "succeeded" {
			return errors.New("successful refund missing remote identity")
		}
		if remote == nil {
			created, err := s.stripe.CreateRefundWithMetadata(ctx, f.Intent, f.Amount, operation)
			if err != nil {
				var apiErr *stripe.APIError
				if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 && apiErr.Status != 409 && apiErr.Status != 429 {
					if _, err := tx.Exec(ctx, `UPDATE captured_payment_refunds SET status='failed',failure=$2,updated_at=now() WHERE id=$1`, f.ID, "Stripe rejected refund: "+apiErr.Code); err != nil {
						return err
					}
					return setCapturedPayment(ctx, tx, f)
				}
				return err
			}
			remote = &created
		}
		if _, err := tx.Exec(ctx, `UPDATE captured_payment_refunds SET reconciled_at=now() WHERE id=$1`, f.ID); err != nil {
			return err
		}
		notify = f.Status != "succeeded" && remote.Status == "succeeded"
		return finalizeCapturedRefund(ctx, tx, f, *remote)
	})
	if err == nil && notify {
		s.deliverRefundCompletionNotice(ctx, id)
	}
	return err
}
func (s *Server) adminCapturedRefund(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Source string `json:"source"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&body); err != nil {
		Error(w, 422, "validation_error", "種類と理由を入力してください")
		return
	}
	body.Reason = strings.TrimSpace(body.Reason)
	if (body.Source != "review" && body.Source != "consultation") || body.Reason == "" || len([]rune(body.Reason)) > 1000 {
		Error(w, 422, "validation_error", "種類と1000文字以内の理由を入力してください")
		return
	}
	if !s.stripe.Configured() {
		Error(w, 409, "payments_disabled", "決済が無効です")
		return
	}
	operation, err := s.prepareCapturedRefund(r.Context(), body.Source, id, admin.ID, body.Reason)
	if err != nil {
		writeCheckoutError(w, err)
		return
	}
	if err = s.processCapturedRefund(r.Context(), operation); err != nil {
		// Ambiguous transport failures remain pending for reconciliation, never create a new operation.
		_, _ = s.pool.Exec(r.Context(), `UPDATE captured_payment_refunds SET failure='Stripe reconciliation required',updated_at=now() WHERE id=$1 AND status='pending'`, operation)
	}
	JSON(w, http.StatusAccepted, map[string]any{"refund_id": operation, "message": "返金を受け付けました。処理状況を再読み込みして確認してください"})
}

// syncCapturedRefund は、Stripe の返金の知らせ（Webhook）をアプリの記録に同期する。
// 全額・一部のどちらの返金も、返金オブジェクト1件ずつ syncStripeRefund に渡す（日次照合と同じ入口）。
// 対象外（未知の支払い・報酬との対応が欠けた支払い・jpy 以外・カード以外）は同期せず、検知に残す。
func (s *Server) syncCapturedRefund(ctx context.Context, ev stripe.Event) error {
	if ev.Type != "refund.created" && ev.Type != "refund.updated" && ev.Type != "refund.failed" && ev.Type != "charge.refunded" {
		return nil
	}
	var obj struct {
		stripe.Refund
		Currency string `json:"currency"`
		Refunds  struct {
			Data []stripe.Refund `json:"data"`
		} `json:"refunds"`
	}
	if err := json.Unmarshal(ev.Data.Object, &obj); err != nil {
		return err
	}
	var refunds []stripe.Refund
	if ev.Type != "charge.refunded" {
		obj.Refund.Currency = obj.Currency
		refunds = []stripe.Refund{obj.Refund}
	} else {
		refunds = obj.Refunds.Data
	}
	for _, remote := range refunds {
		if remote.PaymentIntent == "" {
			remote.PaymentIntent = obj.PaymentIntent
		}
		if remote.Currency == "" {
			remote.Currency = obj.Currency
		}
		if _, err := s.syncStripeRefund(ctx, remote); err != nil {
			return err
		}
	}
	return nil
}

// capturedRefundOperation は、Stripe の返金1件に対応する返金操作を探す。
// 返金 ID が一致するもの、アプリが作った返金の metadata（captured-<操作ID>）が指すもの、
// 返金 ID を受け取れなかった管理画面の処理中の操作（同じ額）の順に見る。
func capturedRefundOperation(ctx context.Context, tx pgx.Tx, source string, id uuid.UUID, remote stripe.Refund) (uuid.UUID, error) {
	var operation uuid.UUID
	var byMetadata *uuid.UUID
	if v, ok := strings.CutPrefix(remote.Metadata["ticket_refund_id"], "captured-"); ok {
		if parsed, err := uuid.Parse(v); err == nil {
			byMetadata = &parsed
		}
	}
	err := tx.QueryRow(ctx, `SELECT id FROM captured_payment_refunds WHERE source=$2 AND payment_id=$3 AND (
 stripe_refund_id=$1 OR id=$4 OR (origin='admin' AND status='pending' AND stripe_refund_id IS NULL AND amount_yen=$5))
 ORDER BY CASE WHEN stripe_refund_id=$1 THEN 0 WHEN id=$4 THEN 1 ELSE 2 END,created_at DESC LIMIT 1`, remote.ID, source, id, byMetadata, remote.Amount).Scan(&operation)
	return operation, err
}

// syncStripeRefund は、Stripe の返金1件をアプリの記録に同期する（Webhook と日次照合の共通の入口、本体 #1097）。
// 同期できたら true。同期の対象外なら false を返し、呼び出し側の検知に残す。
func (s *Server) syncStripeRefund(ctx context.Context, remote stripe.Refund) (bool, error) {
	if remote.PaymentIntent == "" || remote.ID == "" || remote.Currency != "jpy" || remote.Amount <= 0 {
		return false, nil
	}
	var noticeID uuid.UUID
	var synced bool
	err := s.refundTx(ctx, func(tx pgx.Tx) error {
		var err error
		noticeID, err = resolvePendingReviewRefundNotice(ctx, db.New(tx), remote)
		if err != nil || noticeID != uuid.Nil {
			synced = noticeID != uuid.Nil
			return err
		}
		var source string
		var id uuid.UUID
		err = tx.QueryRow(ctx, `SELECT source,id FROM (SELECT 'review' source,id FROM review_payments WHERE payment_intent_id=$1 AND method='card' UNION ALL SELECT 'consultation',id FROM coaching_consultations WHERE payment_intent_id=$1 AND funding='cash') x LIMIT 1`, remote.PaymentIntent).Scan(&source, &id)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		_, status, _, amount, err := capturedPayment(ctx, tx, source, id)
		if err != nil {
			return err
		}
		missing, err := missingReviewRefundLink(ctx, tx, source, id)
		if err != nil {
			return err
		}
		if missing {
			return nil
		}
		operation, err := capturedRefundOperation(ctx, tx, source, id, remote)
		if errors.Is(err, pgx.ErrNoRows) {
			// 記録の無い返金は新しい操作として記録する。失敗・取り消しで届いたものも失敗の記録として残し、
			// 後から順不同で届く成功の知らせで生き返らせない（終端）。
			terminal := remote.Status == "failed" || remote.Status == "canceled"
			// アプリの別の返金処理（期限切れの自動返金など）が作った返金は、その処理の記録が持つ。
			if v := remote.Metadata["ticket_refund_id"]; v != "" && !strings.HasPrefix(v, "captured-") {
				return nil
			}
			if source == "consultation" {
				var legacy bool
				if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM coaching_consultation_refunds WHERE consultation_id=$1 AND (status<>'failed' OR stripe_refund_id=$2))`, id, remote.ID).Scan(&legacy); err != nil {
					return err
				}
				if legacy {
					return nil
				}
			}
			// 支払額を超える同期はしない（Stripe 側は超えて返金できない。超えるなら記録側が食い違っている）。
			refunded, pending, totalsErr := capturedRefundTotals(ctx, tx, source, id)
			if totalsErr != nil {
				return totalsErr
			}
			if status != "captured" && status != "paid" && status != "partially_refunded" && (status != "refund_pending" || pending == 0) {
				return nil
			}
			if !terminal && refunded+pending+remote.Amount > amount {
				return nil
			}
			operation = uuid.New()
			_, err = tx.Exec(ctx, `INSERT INTO captured_payment_refunds(id,source,payment_id,payment_intent_id,amount_yen,reason,origin) VALUES($1,$2,$3,$4,$5,'Stripe Dashboard refund','stripe')`, operation, source, id, remote.PaymentIntent, remote.Amount)
		}
		if err != nil {
			return err
		}
		f, err := loadCapturedRefund(ctx, tx, operation)
		if err != nil {
			return err
		}
		if f.Amount != remote.Amount || (f.Remote != nil && *f.Remote != remote.ID) {
			// 記録と Stripe の額・ID が食い違う返金は同期せず、検知に残す。
			return nil
		}
		if f.Status != "succeeded" && f.Status != "failed" && remote.Status == "succeeded" {
			noticeID = f.ID
		}
		synced = true
		return finalizeCapturedRefund(ctx, tx, f, remote)
	})
	if err != nil {
		return false, err
	}
	if noticeID != uuid.Nil {
		s.deliverRefundCompletionNotice(ctx, noticeID)
	}
	return synced, nil
}

// Retry pending operations in bounded batches from the existing local tick.
func (s *Server) settleCapturedRefunds(ctx context.Context) error {
	if !s.stripe.Configured() {
		return nil
	}
	rows, err := s.pool.Query(ctx, `SELECT id FROM captured_payment_refunds WHERE status='pending' OR (status='succeeded' AND succeeded_at>=now()-interval '37 days') ORDER BY COALESCE(reconciled_at,created_at),id LIMIT 50`)
	if err != nil {
		return err
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.processCapturedRefund(ctx, id); err != nil {
			_, _ = s.pool.Exec(ctx, `UPDATE captured_payment_refunds SET failure='Stripe reconciliation required',reconciled_at=now(),updated_at=now() WHERE id=$1`, id)
		}
	}
	return nil
}

// Money records are admin-only. The parent payment discriminator is the API key.
func (s *Server) adminCapturedPayments(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	page, per, offset := s.queryPage(r)
	coach, refund := r.URL.Query().Get("coach_id"), r.URL.Query().Get("refund_id")
	for _, id := range []string{coach, refund} {
		if id != "" {
			if _, err := uuid.Parse(id); err != nil {
				Error(w, 422, "validation_error", "IDが不正です")
				return
			}
		}
	}
	unpaid := r.URL.Query().Get("unapplied") == "true"
	// Stable ID tie-breaks ensure equal timestamps do not hide older entries.
	rows, err := s.pool.Query(r.Context(), `SELECT json_build_object('source',source,'id',id,'amount_yen',amount_yen,'refunded_yen',refunded_yen,'status',status,'created_at',created_at,'minor',minor) FROM (
 SELECT 'review' source,id,amount_yen,refunded_yen,status,created_at,minor FROM review_payments WHERE method='card' AND status IN ('captured','refund_pending','partially_refunded','refunded')
 UNION ALL SELECT 'consultation',id,price_yen,refunded_yen,payment_state,created_at,minor AND contract_id IS NULL FROM coaching_consultations WHERE funding='cash' AND payment_state IN ('paid','refund_pending','partially_refunded','refunded')) p
 WHERE ($1='' OR EXISTS(SELECT 1 FROM coach_earnings e WHERE e.coach_id::text=$1 AND ((p.source='consultation' AND e.consultation_id=p.id) OR (p.source='review' AND e.id IN(SELECT earning_id FROM review_payment_earning_links WHERE payment_id=p.id)))))
 AND ($2='' OR EXISTS(SELECT 1 FROM captured_payment_refunds f WHERE f.id::text=$2 AND f.source=p.source AND f.payment_id=p.id))
 ORDER BY created_at DESC,id DESC LIMIT $3 OFFSET $4`, coach, refund, per+1, offset)
	if err != nil {
		MapDBError(w, err)
		return
	}
	payments, err := jsonRows(rows)
	if err != nil {
		MapDBError(w, err)
		return
	}
	rows, err = s.pool.Query(r.Context(), `SELECT row_to_json(f) FROM captured_payment_refunds f WHERE ($2='' OR f.id::text=$2) AND ($1='' OR EXISTS(SELECT 1 FROM coach_earnings e WHERE e.coach_id::text=$1 AND ((f.source='consultation' AND e.consultation_id=f.payment_id) OR (f.source='review' AND e.id IN(SELECT earning_id FROM review_payment_earning_links WHERE payment_id=f.payment_id))))) ORDER BY f.created_at DESC,f.id DESC LIMIT $3 OFFSET $4`, coach, refund, per+1, offset)
	if err != nil {
		MapDBError(w, err)
		return
	}
	refunds, err := jsonRows(rows)
	if err != nil {
		MapDBError(w, err)
		return
	}
	rows, err = s.pool.Query(r.Context(), `SELECT json_build_object('id',a.id,'coach_id',a.coach_id,'refund_id',a.refund_id,'kind',a.kind,'amount_yen',a.amount_yen,'applied_yen',a.applied_yen,'canceled_yen',COALESCE(c.amount_yen,0),'remaining_yen',a.amount_yen-a.applied_yen-COALESCE(c.amount_yen,0),
 'cancellation',row_to_json(c),'compensation',(SELECT json_build_object('id',k.id,'earning_id',k.earning_id,'amount_yen',k.amount_yen,'payout_id',e.payout_id) FROM coach_refund_compensations k JOIN coach_earnings e ON e.id=k.earning_id WHERE k.adjustment_id=a.id),
 'applications',COALESCE((SELECT json_agg(json_build_object('id',x.id,'earning_id',x.earning_reference_id,'amount_yen',x.amount_yen,'payout_id',x.payout_id,'reversal',(SELECT row_to_json(v) FROM coach_refund_application_reversals v WHERE v.application_id=x.id))) FROM coach_refund_applications x WHERE x.adjustment_id=a.id),'[]'))
 FROM coach_refund_adjustments a LEFT JOIN coach_refund_cancellations c ON c.adjustment_id=a.id
 WHERE ($1='' OR a.coach_id::text=$1) AND ($2='' OR a.refund_id::text=$2) AND (NOT $3 OR a.amount_yen-a.applied_yen-COALESCE(c.amount_yen,0)>0)
 ORDER BY a.created_at DESC,a.id DESC LIMIT $4 OFFSET $5`, coach, refund, unpaid, per+1, offset)
	if err != nil {
		MapDBError(w, err)
		return
	}
	adjustments, err := jsonRows(rows)
	if err != nil {
		MapDBError(w, err)
		return
	}
	more := len(payments) > int(per) || len(refunds) > int(per) || len(adjustments) > int(per)
	if len(payments) > int(per) {
		payments = payments[:per]
	}
	if len(refunds) > int(per) {
		refunds = refunds[:per]
	}
	if len(adjustments) > int(per) {
		adjustments = adjustments[:per]
	}
	minorRefunds, err := s.minorRefundSummary(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, 200, map[string]any{"payments": payments, "refunds": refunds, "adjustments": adjustments, "page": page, "per": per, "has_more": more, "minor_refunds": minorRefunds})
}

// minorRefundSummary は、18歳未満の本人が自分で払った動画添削・追加相談の返金の件数と金額を、今月（日本時間）と累計で返す。
// 保護者からの申立てで返金したものを含めて、後から数えられるようにする（管理画面の返金 API で返金した分は admin_*）。
func (s *Server) minorRefundSummary(ctx context.Context) (map[string]any, error) {
	now := time.Now().In(jst.Zone)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, jst.Zone)
	far := now.AddDate(100, 0, 0)
	month, err := s.q.SummarizeMinorSelfPayRefunds(ctx, db.SummarizeMinorSelfPayRefundsParams{Since: monthStart, Until: far})
	if err != nil {
		return nil, err
	}
	total, err := s.q.SummarizeMinorSelfPayRefunds(ctx, db.SummarizeMinorSelfPayRefundsParams{Since: time.Unix(0, 0), Until: far})
	if err != nil {
		return nil, err
	}
	return map[string]any{"month": month, "total": total, "month_start": monthStart}, nil
}
func jsonRows(rows pgx.Rows) ([]json.RawMessage, error) {
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var raw json.RawMessage
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		out = append(out, raw)
	}
	return out, rows.Err()
}

type coachRefundOffset struct {
	adjustmentID, earningID uuid.UUID
	amount                  int
}

// planCoachRefundOffsets allocates oldest debt to oldest eligible earnings.
// The send path holds the payout lock and locks the rows it will update.
func planCoachRefundOffsets(ctx context.Context, tx pgx.Tx, coachID *uuid.UUID, lock bool) ([]coachRefundOffset, error) {
	lockClause := ""
	if lock {
		lockClause = " FOR UPDATE"
	}
	rows, err := tx.Query(ctx, `SELECT id,coach_id,amount_yen-applied_yen-COALESCE((SELECT sum(c.amount_yen) FROM coach_refund_cancellations c WHERE c.adjustment_id=coach_refund_adjustments.id),0)::int FROM coach_refund_adjustments WHERE applied_yen<amount_yen AND NOT EXISTS(SELECT 1 FROM coach_refund_cancellations c WHERE c.adjustment_id=coach_refund_adjustments.id) AND coach_id IS NOT NULL AND ($1::uuid IS NULL OR coach_id=$1) ORDER BY created_at,id`+lockClause, coachID)
	if err != nil {
		return nil, err
	}
	type debt struct {
		id, coach uuid.UUID
		remaining int
	}
	debts, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (debt, error) {
		var d debt
		err := row.Scan(&d.id, &d.coach, &d.remaining)
		return d, err
	})
	if err != nil {
		return nil, err
	}

	var plan []coachRefundOffset
	allocated := make(map[uuid.UUID]int)
	for _, d := range debts {
		rows, err := tx.Query(ctx, `SELECT e.id,e.amount_yen-COALESCE((SELECT sum(a.amount_yen) FROM coach_refund_applications a WHERE a.earning_id=e.id AND NOT EXISTS(SELECT 1 FROM coach_refund_application_reversals v WHERE v.application_id=a.id)),0)::int FROM coach_earnings e WHERE e.coach_id=$1 AND e.payout_id IS NULL AND NOT EXISTS (SELECT 1 FROM captured_payment_refunds rf WHERE rf.status='pending' AND ((rf.source='review' AND e.source='review' AND rf.payment_id IN (SELECT payment_id FROM review_payment_earning_links WHERE earning_id=e.id)) OR (rf.source='consultation' AND rf.payment_id=e.consultation_id))) AND e.confirmed_at<date_trunc('month',now() AT TIME ZONE 'Asia/Tokyo') AT TIME ZONE 'Asia/Tokyo' AND NOT EXISTS(SELECT 1 FROM captured_refund_earnings f WHERE f.earning_id=e.id AND f.excluded) ORDER BY e.created_at,e.id`+lockClause, d.coach)
		if err != nil {
			return nil, err
		}
		type available struct {
			id     uuid.UUID
			amount int
		}
		earnings, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (available, error) {
			var e available
			err := row.Scan(&e.id, &e.amount)
			return e, err
		})
		if err != nil {
			return nil, err
		}
		for _, e := range earnings {
			n := min(d.remaining, e.amount-allocated[e.id])
			if n <= 0 {
				continue
			}
			plan = append(plan, coachRefundOffset{d.id, e.id, n})
			allocated[e.id] += n
			d.remaining -= n
			if d.remaining == 0 {
				break
			}
		}
	}
	return plan, nil
}

// Applications without a payout represent a zero/carry month; link on the next positive payout.
func applyCoachRefundOffsets(ctx context.Context, tx pgx.Tx) error {
	plan, err := planCoachRefundOffsets(ctx, tx, nil, true)
	if err != nil {
		return err
	}
	for _, offset := range plan {
		if _, err := tx.Exec(ctx, `INSERT INTO coach_refund_applications(id,adjustment_id,earning_id,amount_yen) VALUES($1,$2,$3,$4)`, uuid.New(), offset.adjustmentID, offset.earningID, offset.amount); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE coach_refund_adjustments SET applied_yen=applied_yen+$2 WHERE id=$1`, offset.adjustmentID, offset.amount); err != nil {
			return err
		}
	}
	return nil
}

// A canceled refund debt is never deleted. Cancel its remaining debit and
// issue a positive earning for every yen already withheld, even if paid out.
func compensateFailedRefund(ctx context.Context, tx pgx.Tx, f capturedRefund) error {
	if _, err := tx.Exec(ctx, `UPDATE captured_refund_earnings SET excluded=false WHERE refund_id=$1`, f.ID); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT id,coach_id,amount_yen,applied_yen FROM coach_refund_adjustments WHERE refund_id=$1 AND kind='refund_offset' FOR UPDATE`, f.ID)
	if err != nil {
		return err
	}
	type debt struct {
		id              uuid.UUID
		coach           *uuid.UUID
		amount, applied int
	}
	var debts []debt
	for rows.Next() {
		var d debt
		if err := rows.Scan(&d.id, &d.coach, &d.amount, &d.applied); err != nil {
			rows.Close()
			return err
		}
		debts = append(debts, d)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, d := range debts {
		if d.amount > d.applied {
			if _, err := tx.Exec(ctx, `INSERT INTO coach_refund_cancellations(id,adjustment_id,refund_id,amount_yen) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, uuid.New(), d.id, f.ID, d.amount-d.applied); err != nil {
				return err
			}
		}
		if d.applied > 0 {
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM coach_refund_compensations WHERE adjustment_id=$1)`, d.id).Scan(&exists); err != nil {
				return err
			}
			if exists {
				continue
			}
			earning := uuid.New()
			if _, err := tx.Exec(ctx, `INSERT INTO coach_earnings(id,coach_id,source,gross_yen,share_percent,amount_yen, confirmed_at) VALUES($1,$2,'refund_compensation',$3,100,$3, now())`, earning, d.coach, d.applied); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO coach_refund_compensations(id,adjustment_id,refund_id,earning_id,amount_yen) VALUES($1,$2,$3,$4,$5)`, uuid.New(), d.id, f.ID, earning, d.applied); err != nil {
				return err
			}
		}
	}
	return nil
}
