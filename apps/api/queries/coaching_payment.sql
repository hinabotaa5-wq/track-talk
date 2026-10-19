-- name: CreateCoachingPayment :one
INSERT INTO coaching_payments (id, contract_id, payer, amount_yen, checkout_session_id, checkout_url, session_expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetReusableCoachingPayment :one
-- まだ使える支払い画面があれば、作り直さずにそれを開く。
SELECT * FROM coaching_payments
WHERE contract_id = $1 AND payer = $2 AND status = 'pending' AND session_expires_at > now() + interval '5 minutes'
ORDER BY created_at DESC
LIMIT 1;

-- name: MarkCoachingPaymentPaid :one
-- Stripe から支払い完了が届いた。同じ知らせが2回届いても、2回目は行が返らない。
UPDATE coaching_payments
SET status = 'paid',
    payment_intent_id = sqlc.arg(payment_intent_id),
    paid_at = now(),
    updated_at = now()
WHERE checkout_session_id = sqlc.arg(checkout_session_id) AND status = 'pending'
RETURNING *;

-- name: StartPaidCoachingContract :one
-- 支払いが済んだら、その時から期間が始まる。18歳未満は保護者の同意があるものだけ。
UPDATE coaching_contracts ct
SET status = 'active',
    starts_at = now(),
    ends_at = now() + make_interval(days => ct.duration_days),
    start_by = NULL,
    updated_at = now()
WHERE ct.id = $1 AND ct.payment_required AND ct.status = 'accepted'
  AND (NOT ct.minor OR EXISTS (
      SELECT 1 FROM coaching_contract_consents c WHERE c.contract_id = ct.id AND c.party = 'guardian'))
RETURNING *;

-- name: GetPaidCoachingPayment :one
-- 返金できる（まだ返金していない）支払い。
SELECT * FROM coaching_payments
WHERE contract_id = $1 AND status = 'paid'
ORDER BY paid_at DESC
LIMIT 1;

-- name: GetLatestCoachingPayment :one
-- 画面に出す支払い。支払い画面を開いただけのものは出さない。
-- 二重の支払いなどで返したものより、期間を始めた支払いを先に出す。
SELECT * FROM coaching_payments
WHERE contract_id = $1 AND status <> 'pending'
ORDER BY refund_reason IS NOT DISTINCT FROM 'not_started', paid_at DESC
LIMIT 1;

-- name: LockPaidCoachingPayment :one
-- 返金を Stripe に送るあいだ、まだ返金していない支払いの行をロックしておく。
-- 同じ支払いの返金が重なったら、あとのほうは前のが記録し終えるのを待ってから行が返らなくなる。
SELECT * FROM coaching_payments WHERE id = $1 AND status = 'paid' FOR UPDATE;

-- name: RecordCoachingRefund :one
-- 返金は1つの支払いに1回だけ。
UPDATE coaching_payments
SET refunded_yen = sqlc.arg(refunded_yen)::int,
    status = CASE WHEN sqlc.arg(refunded_yen)::int >= amount_yen THEN 'refunded' ELSE 'partially_refunded' END,
    refund_reason = sqlc.arg(refund_reason)::text,
    refunded_at = now(),
    updated_at = now()
WHERE id = sqlc.arg(id) AND status = 'paid'
RETURNING *;

-- name: RefundCoachingContract :one
-- 返金して、期間の途中で閉じる。
UPDATE coaching_contracts
SET status = 'refunded',
    ends_at = LEAST(ends_at, now()),
    updated_at = now()
WHERE id = $1 AND status IN ('active', 'suspended')
RETURNING *;

-- name: ListSilentPaidContracts :many
-- 支払いが済んで始まってから72時間、コーチが1通も書いていないもの。全額を返金して閉じる。
SELECT ct.* FROM coaching_contracts ct
WHERE ct.status = 'active' AND ct.payment_required
  AND ct.starts_at <= now() - interval '72 hours'
  AND NOT EXISTS (SELECT 1 FROM coaching_messages m WHERE m.contract_id = ct.id AND m.sender_role = 'coach')
  AND EXISTS (SELECT 1 FROM coaching_payments pay WHERE pay.contract_id = ct.id AND pay.status = 'paid')
LIMIT 50;

-- name: ListUnstartedPaidPayments :many
-- 支払いは済んだのに期間が始まらなかったもの（期限を過ぎてから払った、二重に払った）。返金し損ねたものを拾う。
SELECT pay.* FROM coaching_payments pay
JOIN coaching_contracts ct ON ct.id = pay.contract_id
WHERE pay.status = 'paid'
  AND pay.paid_at <= now() - interval '1 minute'
  AND (ct.status NOT IN ('active', 'suspended', 'ended')
       OR EXISTS (SELECT 1 FROM coaching_payments o WHERE o.contract_id = pay.contract_id AND o.id <> pay.id
                  AND o.status <> 'pending' AND o.paid_at < pay.paid_at))
LIMIT 50;

-- name: HasGuardianContractConsent :one
SELECT EXISTS (
    SELECT 1 FROM coaching_contract_consents WHERE contract_id = $1 AND party = 'guardian'
)::boolean;
