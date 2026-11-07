-- name: EnqueueReviewRefundNotice :exec
INSERT INTO refund_completion_notices (id, recipient_id, kind, url, pending_stripe_refund_id)
SELECT sqlc.arg(operation_id), pay.payer_id, sqlc.arg(kind),
       COALESCE('/coaching/requests/' || r.id::text, '/me/coaching/requests'),
       sqlc.narg(pending_stripe_refund_id)
FROM review_payments pay
LEFT JOIN review_requests r ON r.id = pay.request_id AND r.requester_id = pay.payer_id
    AND r.status <> 'awaiting_payment'
WHERE pay.id = sqlc.arg(payment_id) AND pay.payer_id IS NOT NULL
ON CONFLICT (id) DO NOTHING;

-- name: EnqueueConsultationRefundNotice :exec
INSERT INTO refund_completion_notices (id, recipient_id, kind, url)
VALUES ($1, $2, 'consultation_refund', $3)
ON CONFLICT (id) DO NOTHING;

-- name: ListDueRefundCompletionNotices :many
SELECT id FROM refund_completion_notices WHERE pending_stripe_refund_id IS NULL AND next_attempt_at <= now()
ORDER BY next_attempt_at, id LIMIT 50;

-- name: LockRefundCompletionNotice :one
SELECT * FROM refund_completion_notices WHERE id = $1 AND pending_stripe_refund_id IS NULL FOR UPDATE SKIP LOCKED;

-- name: DeleteRefundCompletionNotice :exec
DELETE FROM refund_completion_notices WHERE id = $1;

-- name: DeferRefundCompletionNotice :exec
UPDATE refund_completion_notices SET next_attempt_at = now() + interval '1 minute' WHERE id = $1;

-- name: InsertRefundCompletionBell :execrows
INSERT INTO notifications (id, recipient_id, kind, title, body, url)
VALUES ($1, $2, 'message', $3, $4, $5)
ON CONFLICT (id) DO NOTHING;

-- name: ConfirmPendingReviewRefundNotice :one
UPDATE refund_completion_notices n
SET pending_stripe_refund_id = NULL, next_attempt_at = now()
FROM review_payments pay
WHERE n.id = pay.id
  AND n.pending_stripe_refund_id = sqlc.arg(stripe_refund_id)
  AND pay.payment_intent_id = sqlc.arg(payment_intent_id)
  AND pay.amount_yen = sqlc.arg(amount_yen)
RETURNING n.id;

-- name: CancelPendingReviewRefundNotice :exec
DELETE FROM refund_completion_notices n USING review_payments pay
WHERE n.id = pay.id
  AND n.pending_stripe_refund_id = sqlc.arg(stripe_refund_id)
  AND pay.payment_intent_id = sqlc.arg(payment_intent_id)
  AND pay.amount_yen = sqlc.arg(amount_yen);
