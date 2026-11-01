-- name: ListTicketProducts :many
SELECT * FROM coaching_ticket_products WHERE active ORDER BY quantity;

-- name: GetTicketProduct :one
SELECT * FROM coaching_ticket_products WHERE id = $1 AND active;

-- name: GetTicketPurchaseByKey :one
SELECT * FROM coaching_ticket_purchases WHERE owner_id = $1 AND request_key = $2;

-- name: GetTicketPurchase :one
SELECT * FROM coaching_ticket_purchases WHERE id = $1;

-- name: LockTicketPurchase :one
SELECT * FROM coaching_ticket_purchases WHERE id = $1 FOR UPDATE;

-- name: CreateTicketPurchase :one
INSERT INTO coaching_ticket_purchases (id, owner_id, request_key, product_id, quantity, amount_yen, session_expires_at, return_path)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING *;

-- name: SaveTicketCheckout :exec
UPDATE coaching_ticket_purchases SET checkout_session_id = $2, checkout_url = $3
WHERE id = $1 AND (checkout_session_id IS NULL OR checkout_session_id = $2);

-- name: MarkTicketPurchasePaid :one
UPDATE coaching_ticket_purchases SET status = 'paid', payment_intent_id = $2,
    checkout_session_id = COALESCE(checkout_session_id, $3), paid_at = now()
WHERE id = $1 AND status = 'pending' RETURNING *;

-- name: CreateTicketLot :exec
INSERT INTO coaching_ticket_lots (id, owner_id, quantity, available) VALUES ($1,$2,$3,$3);

-- name: TicketBalance :one
SELECT COALESCE(sum(available),0)::bigint AS available, COALESCE(sum(reserved),0)::bigint AS reserved
FROM coaching_ticket_lots WHERE owner_id = $1;

-- name: ListTicketLots :many
SELECT * FROM coaching_ticket_lots WHERE owner_id = $1 AND available > 0 ORDER BY created_at,id FOR UPDATE;

-- name: LockTicketLot :one
SELECT * FROM coaching_ticket_lots WHERE id = $1 FOR UPDATE;

-- name: MoveTicketLot :exec
UPDATE coaching_ticket_lots SET available = available + sqlc.arg(available_delta)::integer,
    reserved = reserved + sqlc.arg(reserved_delta)::integer,
    consumed = consumed + sqlc.arg(consumed_delta)::integer,
    refunded = refunded + sqlc.arg(refunded_delta)::integer
WHERE id = sqlc.arg(id);

-- name: InsertTicketLedger :exec
INSERT INTO coaching_ticket_ledger (id, lot_id, operation_key, kind, available_delta, reserved_delta, consumed_delta, refunded_delta)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8);

-- name: CreateTicketReservation :one
INSERT INTO coaching_ticket_reservations (id,owner_id,request_id,quantity,nominal_yen,share_percent)
VALUES ($1,$2,$3,$4,$5,$6) RETURNING *;

-- name: CreateTicketAllocation :exec
INSERT INTO coaching_ticket_allocations(reservation_id,lot_id,quantity) VALUES ($1,$2,$3);

-- name: GetTicketReservationForReview :one
SELECT * FROM coaching_ticket_reservations WHERE request_id = $1;

-- name: LockTicketReservation :one
SELECT * FROM coaching_ticket_reservations WHERE id = $1 FOR UPDATE;

-- name: ListTicketAllocations :many
SELECT a.* FROM coaching_ticket_allocations a JOIN coaching_ticket_lots l ON l.id=a.lot_id WHERE a.reservation_id = $1 ORDER BY l.created_at,l.id;

-- name: SettleTicketReservation :exec
UPDATE coaching_ticket_reservations SET status = $2, settled_at = now() WHERE id = $1 AND status = 'reserved';

-- name: ListTicketReservationsToRelease :many
SELECT t.* FROM coaching_ticket_reservations t LEFT JOIN review_requests r ON r.id = t.request_id
WHERE t.status = 'reserved' AND t.consultation_id IS NULL AND (r.id IS NULL OR r.status = 'expired') ORDER BY t.created_at LIMIT 100;

-- name: LockTicketReview :one
SELECT * FROM review_requests WHERE id = $1 FOR UPDATE;

-- name: RecordTicketReviewEarnings :exec
INSERT INTO coach_earnings (id,coach_id,source,review_request_id,gross_yen,share_percent,amount_yen)
SELECT gen_random_uuid(),r.coach_id,'review',r.id,t.nominal_yen,t.share_percent,t.nominal_yen*t.share_percent/100
FROM coaching_ticket_reservations t JOIN review_requests r ON r.id = t.request_id
WHERE t.id = $1 AND t.status = 'consumed' AND r.returned_at IS NOT NULL AND r.coach_id IS NOT NULL
ON CONFLICT (review_request_id) DO NOTHING;

-- name: CreateTicketRefund :one
INSERT INTO coaching_ticket_refunds (id,purchase_id,quantity,amount_yen) VALUES ($1,$2,$3,$4) RETURNING *;

-- name: LockTicketRefund :one
SELECT * FROM coaching_ticket_refunds WHERE id = $1 FOR UPDATE;

-- name: GetPendingTicketRefund :one
SELECT * FROM coaching_ticket_refunds WHERE purchase_id = $1 AND status = 'pending';

-- name: SaveTicketRefund :exec
UPDATE coaching_ticket_refunds SET stripe_refund_id = $2,status = $3,updated_at = now() WHERE id = $1;

-- name: FinishTicketPurchaseRefund :exec
UPDATE coaching_ticket_purchases SET refunded_yen = refunded_yen + sqlc.arg(amount_yen)::integer,
    status = CASE WHEN refunded_yen + sqlc.arg(amount_yen)::integer = amount_yen THEN 'refunded' ELSE 'partially_refunded' END
WHERE id = sqlc.arg(id);

-- name: ListPendingTicketRefunds :many
SELECT * FROM coaching_ticket_refunds WHERE status = 'pending' ORDER BY updated_at LIMIT 50;

-- name: TouchTicketRefund :exec
UPDATE coaching_ticket_refunds SET updated_at = now() WHERE id = $1;

-- name: ListMyTicketPurchases :many
SELECT p.id,p.request_key,p.quantity,p.amount_yen,p.status,p.refunded_yen,p.created_at,p.paid_at,
    COALESCE(l.available,0)::integer AS available,COALESCE(l.reserved,0)::integer AS reserved,
    EXISTS(SELECT 1 FROM coaching_ticket_refunds f WHERE f.purchase_id=p.id AND f.status='pending')::boolean AS refund_pending
FROM coaching_ticket_purchases p LEFT JOIN coaching_ticket_lots l ON l.id = p.id
WHERE p.owner_id = $1 ORDER BY p.created_at DESC LIMIT 100;

-- name: ListMyTicketLedger :many
SELECT e.* FROM coaching_ticket_ledger e JOIN coaching_ticket_lots l ON l.id = e.lot_id
WHERE l.owner_id = $1 ORDER BY e.created_at DESC,e.id DESC LIMIT 100;

-- name: AdminTicketPurchases :many
SELECT p.id,p.owner_id,p.quantity,p.amount_yen,p.status,p.refunded_yen,p.created_at,p.paid_at,
    COALESCE(l.available,0)::integer AS available,COALESCE(l.reserved,0)::integer AS reserved,
    pr.nickname,
    EXISTS(SELECT 1 FROM coaching_ticket_refunds r WHERE r.purchase_id = p.id AND r.status = 'pending')::boolean AS refund_pending
FROM coaching_ticket_purchases p LEFT JOIN coaching_ticket_lots l ON l.id = p.id
LEFT JOIN profiles pr ON pr.id = p.owner_id ORDER BY p.created_at DESC LIMIT 200;

-- name: TicketReconciliation :many
SELECT l.id FROM coaching_ticket_lots l LEFT JOIN coaching_ticket_ledger e ON e.lot_id = l.id
GROUP BY l.id HAVING l.available <> COALESCE(sum(e.available_delta),0)
 OR l.reserved <> COALESCE(sum(e.reserved_delta),0) OR l.consumed <> COALESCE(sum(e.consumed_delta),0)
 OR l.refunded <> COALESCE(sum(e.refunded_delta),0);

-- name: HasOpenTicketAccount :one
SELECT (EXISTS(SELECT 1 FROM coaching_ticket_lots l WHERE l.owner_id = $1 AND (l.available > 0 OR l.reserved > 0))
 OR EXISTS(SELECT 1 FROM coaching_ticket_purchases t WHERE t.owner_id = $1 AND t.status = 'pending' AND t.session_expires_at > now())
 OR EXISTS(SELECT 1 FROM coaching_ticket_refunds f JOIN coaching_ticket_purchases p ON p.id=f.purchase_id WHERE p.owner_id=$1 AND f.status='pending'))::boolean;

-- name: SumMonthlyCoachingSpend :one
SELECT COALESCE(sum(x.amount),0)::bigint FROM (
 SELECT amount_yen::bigint AS amount FROM review_payments WHERE payer_id = sqlc.arg(owner_id)::uuid
 AND request_id IS DISTINCT FROM sqlc.arg(exclude_review_id)::uuid
 AND created_at >= date_trunc('month',now() AT TIME ZONE 'Asia/Tokyo') AT TIME ZONE 'Asia/Tokyo'
 AND (status IN ('authorized','captured','refund_pending') OR (status='pending' AND session_expires_at>now()))
 UNION ALL
 SELECT (pay.amount_yen-pay.refunded_yen)::bigint FROM coaching_payments pay JOIN coaching_contracts ct ON ct.id=pay.contract_id
 WHERE ct.requester_id = sqlc.arg(owner_id)::uuid AND ct.id IS DISTINCT FROM sqlc.arg(exclude_contract_id)::uuid
 AND pay.created_at >= date_trunc('month',now() AT TIME ZONE 'Asia/Tokyo') AT TIME ZONE 'Asia/Tokyo'
 AND (pay.status IN ('paid','partially_refunded') OR (pay.status='pending' AND pay.session_expires_at>now()))
 UNION ALL
 SELECT (amount_yen-refunded_yen)::bigint FROM coaching_ticket_purchases WHERE owner_id = sqlc.arg(owner_id)::uuid
 AND id IS DISTINCT FROM sqlc.arg(exclude_purchase_id)::uuid
 AND COALESCE(paid_at,created_at) >= date_trunc('month',now() AT TIME ZONE 'Asia/Tokyo') AT TIME ZONE 'Asia/Tokyo'
 AND (status IN ('paid','partially_refunded') OR (status='pending' AND session_expires_at>now()))
 UNION ALL
 SELECT c.price_yen::bigint FROM coaching_consultations c WHERE c.requester_id=sqlc.arg(owner_id)::uuid
 AND c.funding='cash' AND c.id IS DISTINCT FROM sqlc.arg(exclude_consultation_id)::uuid
 AND COALESCE(c.paid_at,c.updated_at)>=date_trunc('month',now() AT TIME ZONE 'Asia/Tokyo') AT TIME ZONE 'Asia/Tokyo'
 AND (c.payment_state IN ('paid','refund_pending') OR (c.status='awaiting_payment' AND c.expires_at>now() AND (c.session_expires_at IS NULL OR c.session_expires_at>now())))
) x;
