package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
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

func setCapturedPayment(ctx context.Context, tx pgx.Tx, f capturedRefund, status string) error {
	var err error
	if f.Source == "review" {
		_, err = tx.Exec(ctx, `UPDATE review_payments SET status=$2,refunded_at=CASE WHEN $2='refunded' THEN now() ELSE refunded_at END,updated_at=now() WHERE id=$1`, f.PaymentID, status)
	} else {
		if status == "captured" {
			status = "paid"
		}
		_, err = tx.Exec(ctx, `UPDATE coaching_consultations SET payment_state=$2,updated_at=now() WHERE id=$1`, f.PaymentID, status)
	}
	if err == nil && status == "paid" && f.Source == "consultation" {
		return db.New(tx).RecordConsultationEarnings(ctx, f.PaymentID)
	}
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
		err = tx.QueryRow(ctx, `SELECT id FROM captured_payment_refunds WHERE source=$1 AND payment_id=$2 AND status<>'failed'`, source, id).Scan(&operation)
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
		if (status != "captured" && !(source == "consultation" && status == "paid")) || intent == "" || method != "card" {
			return &ticketRuleError{"not_refundable", "確定済みのカード支払いだけ返金できます"}
		}
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
		return setCapturedPayment(ctx, tx, capturedRefund{Source: source, PaymentID: id}, "refund_pending")
	})
	return operation, err
}

// On final success, exclude unsent earnings, preserving historical payouts.
func finalizeCapturedRefund(ctx context.Context, tx pgx.Tx, f capturedRefund, remote stripe.Refund) error {
	if f.Status == "succeeded" || f.Status == "failed" {
		return nil
	}
	if f.Remote != nil && *f.Remote != remote.ID {
		return errors.New("refund identity mismatch")
	}
	if remote.ID == "" || remote.Amount != f.Amount {
		return errors.New("full refund amount mismatch")
	}
	status := "pending"
	switch remote.Status {
	case "succeeded":
		status = "succeeded"
	case "failed", "canceled":
		status = "failed"
	}
	if _, err := tx.Exec(ctx, `UPDATE captured_payment_refunds SET status=$2,stripe_refund_id=$3,failure=$4,updated_at=now() WHERE id=$1`, f.ID, status, remote.ID, func() any {
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
	if status == "failed" {
		return setCapturedPayment(ctx, tx, f, "captured")
	}
	if status == "pending" {
		return setCapturedPayment(ctx, tx, f, "refund_pending")
	}
	if err := setCapturedPayment(ctx, tx, f, "refunded"); err != nil {
		return err
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
		if _, err = tx.Exec(ctx, `INSERT INTO captured_refund_earnings(earning_id,refund_id,excluded) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, e.id, f.ID, e.payout == nil); err != nil {
			return err
		}
		if e.payout != nil {
			if _, err = tx.Exec(ctx, `INSERT INTO coach_refund_adjustments(id,refund_id,earning_id,coach_id,amount_yen) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, uuid.New(), f.ID, e.id, e.coach, e.amount); err != nil {
				return err
			}
		} else {
			// If this earning already paid down older debt, restore that debt: the
			// refunded earning can no longer be used as consideration for an offset.
			if _, err = tx.Exec(ctx, `UPDATE coach_refund_adjustments a SET applied_yen=a.applied_yen-x.n FROM (SELECT adjustment_id,sum(amount_yen)::int n FROM coach_refund_applications WHERE earning_id=$1 AND payout_id IS NULL GROUP BY adjustment_id) x WHERE a.id=x.adjustment_id`, e.id); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `DELETE FROM coach_refund_applications WHERE earning_id=$1 AND payout_id IS NULL`, e.id); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *Server) processCapturedRefund(ctx context.Context, id uuid.UUID) error {
	return s.refundTx(ctx, func(tx pgx.Tx) error {
		f, err := loadCapturedRefund(ctx, tx, id)
		if err != nil {
			return err
		}
		if f.Status != "pending" {
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
		if remote == nil {
			created, err := s.stripe.CreateRefundWithMetadata(ctx, f.Intent, f.Amount, operation)
			if err != nil {
				var apiErr *stripe.APIError
				if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 && apiErr.Status != 409 && apiErr.Status != 429 {
					if _, err := tx.Exec(ctx, `UPDATE captured_payment_refunds SET status='failed',failure=$2,updated_at=now() WHERE id=$1`, f.ID, "Stripe rejected refund: "+apiErr.Code); err != nil {
						return err
					}
					return setCapturedPayment(ctx, tx, f, "captured")
				}
				return err
			}
			remote = &created
		}
		return finalizeCapturedRefund(ctx, tx, f, *remote)
	})
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

// Reconcile only supported full refunds. Unknown/partial events retain existing alerts.
func (s *Server) syncCapturedRefund(ctx context.Context, ev stripe.Event) error {
	if ev.Type != "refund.created" && ev.Type != "charge.refunded" {
		return nil
	}
	var obj struct {
		ID            string `json:"id"`
		PaymentIntent string `json:"payment_intent"`
		Amount        int    `json:"amount"`
		Currency      string `json:"currency"`
		Status        string `json:"status"`
		Refunds       struct {
			Data []stripe.Refund `json:"data"`
		} `json:"refunds"`
	}
	if err := json.Unmarshal(ev.Data.Object, &obj); err != nil {
		return err
	}
	var refunds []stripe.Refund
	if ev.Type == "refund.created" {
		refunds = []stripe.Refund{{ID: obj.ID, PaymentIntent: obj.PaymentIntent, Amount: obj.Amount, Status: obj.Status}}
	} else {
		refunds = obj.Refunds.Data
	}
	for _, remote := range refunds {
		if remote.PaymentIntent == "" {
			remote.PaymentIntent = obj.PaymentIntent
		}
		if remote.PaymentIntent == "" || remote.ID == "" || obj.Currency != "jpy" {
			continue
		}
		err := s.refundTx(ctx, func(tx pgx.Tx) error {
			var source string
			var id uuid.UUID
			err := tx.QueryRow(ctx, `SELECT source,id FROM (SELECT 'review' source,id FROM review_payments WHERE payment_intent_id=$1 AND method='card' UNION ALL SELECT 'consultation',id FROM coaching_consultations WHERE payment_intent_id=$1 AND funding='cash') x LIMIT 1`, remote.PaymentIntent).Scan(&source, &id)
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
			if amount != remote.Amount {
				return nil
			}
			var operation uuid.UUID
			err = tx.QueryRow(ctx, `SELECT id FROM captured_payment_refunds WHERE stripe_refund_id=$1 OR (source=$2 AND payment_id=$3 AND status<>'failed') ORDER BY CASE WHEN stripe_refund_id=$1 THEN 0 ELSE 1 END,created_at DESC LIMIT 1`, remote.ID, source, id).Scan(&operation)
			if errors.Is(err, pgx.ErrNoRows) {
				if status != "captured" && status != "paid" {
					return nil
				}
				operation = uuid.New()
				_, err = tx.Exec(ctx, `INSERT INTO captured_payment_refunds(id,source,payment_id,payment_intent_id,amount_yen,reason,origin) VALUES($1,$2,$3,$4,$5,'Stripe Dashboard refund','stripe')`, operation, source, id, remote.PaymentIntent, amount)
			}
			if err != nil {
				return err
			}
			f, err := loadCapturedRefund(ctx, tx, operation)
			if err != nil {
				return err
			}
			return finalizeCapturedRefund(ctx, tx, f, remote)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// Retry pending operations in bounded batches from the existing local tick.
func (s *Server) settleCapturedRefunds(ctx context.Context) error {
	if !s.stripe.Configured() {
		return nil
	}
	rows, err := s.pool.Query(ctx, `SELECT id FROM captured_payment_refunds WHERE status='pending' ORDER BY updated_at LIMIT 50`)
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
			_, _ = s.pool.Exec(ctx, `UPDATE captured_payment_refunds SET failure='Stripe reconciliation required',updated_at=now() WHERE id=$1`, id)
		}
	}
	return nil
}

// Money records are admin-only. The parent payment discriminator is the API key.
func (s *Server) adminCapturedPayments(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	rows, err := s.pool.Query(r.Context(), `SELECT json_build_object('source',source,'id',id,'amount_yen',amount_yen,'status',status,'created_at',created_at) FROM (
 SELECT 'review' source,id,amount_yen,status,created_at FROM review_payments WHERE method='card' AND status IN ('captured','refund_pending','refunded')
 UNION ALL SELECT 'consultation',id,price_yen,payment_state,created_at FROM coaching_consultations WHERE funding='cash' AND payment_state IN ('paid','refund_pending','refunded')) p ORDER BY created_at DESC LIMIT 200`)
	if err != nil {
		MapDBError(w, err)
		return
	}
	payments, err := jsonRows(rows)
	if err != nil {
		MapDBError(w, err)
		return
	}
	rows, err = s.pool.Query(r.Context(), `SELECT row_to_json(f) FROM captured_payment_refunds f ORDER BY created_at DESC LIMIT 200`)
	if err != nil {
		MapDBError(w, err)
		return
	}
	refunds, err := jsonRows(rows)
	if err != nil {
		MapDBError(w, err)
		return
	}
	rows, err = s.pool.Query(r.Context(), `SELECT json_build_object('id',a.id,'coach_id',a.coach_id,'refund_id',a.refund_id,'amount_yen',a.amount_yen,'applied_yen',a.applied_yen,'remaining_yen',a.amount_yen-a.applied_yen,'applications',COALESCE((SELECT json_agg(x) FROM coach_refund_applications x WHERE x.adjustment_id=a.id),'[]')) FROM coach_refund_adjustments a ORDER BY a.created_at DESC LIMIT 200`)
	if err != nil {
		MapDBError(w, err)
		return
	}
	adjustments, err := jsonRows(rows)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, 200, map[string]any{"payments": payments, "refunds": refunds, "adjustments": adjustments})
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

// Apply oldest debt to oldest eligible earnings under LockCoachPayouts.
// Applications without a payout represent a zero/carry month; link on the next positive payout.
func applyCoachRefundOffsets(ctx context.Context, tx pgx.Tx) error {
	rows, err := tx.Query(ctx, `SELECT id,coach_id,amount_yen-applied_yen FROM coach_refund_adjustments WHERE applied_yen<amount_yen AND coach_id IS NOT NULL ORDER BY created_at,id FOR UPDATE`)
	if err != nil {
		return err
	}
	type debt struct {
		id, coach uuid.UUID
		remaining int
	}
	var debts []debt
	for rows.Next() {
		var d debt
		if err := rows.Scan(&d.id, &d.coach, &d.remaining); err != nil {
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
		rows, err := tx.Query(ctx, `SELECT e.id,e.amount_yen-COALESCE((SELECT sum(a.amount_yen) FROM coach_refund_applications a WHERE a.earning_id=e.id),0)::int FROM coach_earnings e WHERE e.coach_id=$1 AND e.payout_id IS NULL AND NOT EXISTS (SELECT 1 FROM captured_payment_refunds rf WHERE rf.status='pending' AND ((rf.source='review' AND e.source='review' AND rf.payment_id IN (SELECT payment_id FROM review_payment_earning_links WHERE earning_id=e.id)) OR (rf.source='consultation' AND rf.payment_id=e.consultation_id))) AND e.created_at<date_trunc('month',now() AT TIME ZONE 'Asia/Tokyo') AT TIME ZONE 'Asia/Tokyo' AND NOT EXISTS(SELECT 1 FROM captured_refund_earnings f WHERE f.earning_id=e.id AND f.excluded) ORDER BY e.created_at,e.id FOR UPDATE`, d.coach)
		if err != nil {
			return err
		}
		type available struct {
			id     uuid.UUID
			amount int
		}
		var earns []available
		for rows.Next() {
			var e available
			if err := rows.Scan(&e.id, &e.amount); err != nil {
				rows.Close()
				return err
			}
			earns = append(earns, e)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, e := range earns {
			n := min(d.remaining, e.amount)
			if n <= 0 {
				continue
			}
			if _, err := tx.Exec(ctx, `INSERT INTO coach_refund_applications(id,adjustment_id,earning_id,amount_yen) VALUES($1,$2,$3,$4)`, uuid.New(), d.id, e.id, n); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE coach_refund_adjustments SET applied_yen=applied_yen+$2 WHERE id=$1`, d.id, n); err != nil {
				return err
			}
			d.remaining -= n
			if d.remaining == 0 {
				break
			}
		}
	}
	return nil
}
