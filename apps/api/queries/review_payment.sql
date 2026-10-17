-- name: CreateReviewPayment :one
INSERT INTO review_payments (id, request_id, payer_id, method, amount_yen, checkout_session_id, checkout_url, session_expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetReusableReviewPayment :one
-- まだ使える支払い画面があれば、作り直さずにそれを開く。
SELECT * FROM review_payments
WHERE request_id = $1 AND method = $2 AND status = 'pending' AND session_expires_at > now() + interval '5 minutes'
ORDER BY created_at DESC
LIMIT 1;

-- name: GetReviewPaymentBySession :one
SELECT * FROM review_payments
WHERE checkout_session_id = $1;

-- name: MarkReviewPaymentPaid :one
-- Stripe から支払い（カードは仮押さえ）が届いた。同じ知らせが2回届いても、2回目は行が返らない。
UPDATE review_payments
SET status = sqlc.arg(status)::text,
    payment_intent_id = sqlc.arg(payment_intent_id),
    authorized_at = now(),
    captured_at = CASE WHEN sqlc.arg(status)::text = 'captured' THEN now() END,
    updated_at = now()
WHERE checkout_session_id = sqlc.arg(checkout_session_id) AND status = 'pending'
RETURNING *;

-- name: StartPaidReviewRequest :one
-- 支払いが届いたら、割り当ての待ち行列に入れる。目安を過ぎていたら入れない（行が返らない）。
UPDATE review_requests
SET status = 'queued',
    updated_at = now()
WHERE id = $1 AND status = 'awaiting_payment' AND due_at > now()
RETURNING *;

-- name: MarkReviewPaymentCaptured :one
UPDATE review_payments
SET status = 'captured',
    captured_at = now(),
    updated_at = now()
WHERE id = $1 AND status = 'authorized'
RETURNING *;

-- name: MarkReviewPaymentReleased :one
-- 仮押さえなら取り消し、確定済み（PayPay）なら返金した。
UPDATE review_payments
SET status = CASE WHEN status = 'authorized' THEN 'canceled' ELSE 'refunded' END,
    canceled_at = CASE WHEN status = 'authorized' THEN now() END,
    refunded_at = CASE WHEN status = 'captured' THEN now() END,
    updated_at = now()
WHERE id = $1 AND status IN ('authorized', 'captured')
RETURNING *;

-- name: ListReviewPaymentsToCapture :many
-- 返却された依頼の、まだ確定していない仮押さえ。
SELECT pay.* FROM review_payments pay
JOIN review_requests r ON r.id = pay.request_id
WHERE pay.status = 'authorized' AND r.returned_at IS NOT NULL
LIMIT 50;

-- name: ListReviewPaymentsToRelease :many
-- 取り消すか返金する支払い: 目安を過ぎた依頼のもの、支払いが遅れて始められなかったもの、二重の支払い、依頼が消えたもの。
SELECT pay.* FROM review_payments pay
LEFT JOIN review_requests r ON r.id = pay.request_id
WHERE pay.status IN ('authorized', 'captured')
  AND pay.authorized_at <= now() - interval '1 minute'
  AND (r.id IS NULL
       OR r.status IN ('expired', 'awaiting_payment')
       OR (r.returned_at IS NULL AND EXISTS (
           SELECT 1 FROM review_payments o
           WHERE o.request_id = pay.request_id AND o.id <> pay.id
             AND o.status IN ('authorized', 'captured') AND o.authorized_at < pay.authorized_at)))
LIMIT 50;

-- name: DeleteUnpaidReviewRequests :many
-- 払わないまま支払い画面が閉じた依頼を消す。同じ動画で送り直せるようにする。
DELETE FROM review_requests r
WHERE r.status = 'awaiting_payment'
  AND r.created_at <= now() - interval '2 hours'
  AND NOT EXISTS (SELECT 1 FROM review_payments pay WHERE pay.request_id = r.id AND pay.status IN ('authorized', 'captured'))
RETURNING r.id;

-- name: SumMonthlyReviewPayments :one
-- 18歳未満の月の上限に使う、その人の今月（日本時間）の支払い。開いたままの支払い画面も数える。
SELECT COALESCE(sum(amount_yen), 0)::int FROM review_payments
WHERE payer_id = sqlc.arg(payer_id)
  AND request_id IS DISTINCT FROM sqlc.arg(exclude_request_id)::uuid
  AND created_at >= date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo') AT TIME ZONE 'Asia/Tokyo'
  AND (status IN ('authorized', 'captured') OR (status = 'pending' AND session_expires_at > now()));

-- name: GetLatestReviewPayment :one
-- 画面に出す支払い。支払い画面を開いただけのものは出さない。
SELECT * FROM review_payments
WHERE request_id = $1 AND status <> 'pending'
ORDER BY authorized_at
LIMIT 1;

-- name: DeleteAwaitingReviewRequest :exec
DELETE FROM review_requests
WHERE id = $1 AND status = 'awaiting_payment';
