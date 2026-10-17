-- name: SetCoachStripeAccount :one
-- 受け取り用の Stripe アカウントを紐づける。すでにあれば、そのまま返す。
UPDATE coach_profiles
SET stripe_account_id = COALESCE(stripe_account_id, sqlc.arg(stripe_account_id)),
    updated_at = now()
WHERE profile_id = sqlc.arg(profile_id)
RETURNING *;

-- name: SetCoachPayoutsEnabled :one
UPDATE coach_profiles
SET payouts_enabled = sqlc.arg(payouts_enabled),
    updated_at = now()
WHERE profile_id = sqlc.arg(profile_id)
RETURNING *;

-- name: SetCoachReducedFee :one
-- 運営が「初年10%」を付けたり外したりする。付けた時から1年。
UPDATE coach_profiles
SET reduced_fee_until = CASE WHEN sqlc.arg(enabled)::boolean THEN now() + interval '1 year' END,
    updated_at = now()
WHERE profile_id = sqlc.arg(profile_id) AND status = 'approved'
RETURNING *;

-- name: InsertReviewEarnings :many
-- 返却して代金が確定した添削の、コーチの取り分を記録する。
INSERT INTO coach_earnings (id, coach_id, source, review_request_id, gross_yen, share_percent, amount_yen)
SELECT gen_random_uuid(), r.coach_id, 'review', r.id, pay.amount_yen, sqlc.arg(share_percent)::smallint,
       pay.amount_yen * sqlc.arg(share_percent)::int / 100
FROM review_payments pay
JOIN review_requests r ON r.id = pay.request_id
WHERE pay.status = 'captured' AND r.returned_at IS NOT NULL AND r.coach_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM coach_earnings e WHERE e.review_request_id = r.id)
LIMIT 100
ON CONFLICT DO NOTHING
RETURNING *;

-- name: InsertPersonalEarnings :many
-- 期間が終わった（途中で返金して閉じた）パーソナル指導の、手数料を引いたコーチの取り分を記録する。
-- 手数料は20%、初年10%のコーチは、その期間中に始まった指導が10%。
INSERT INTO coach_earnings (id, coach_id, source, contract_id, gross_yen, share_percent, amount_yen)
SELECT gen_random_uuid(), x.coach_id, 'personal', x.contract_id, x.gross, x.share, x.gross * x.share / 100
FROM (
    SELECT DISTINCT ON (ct.id)
        ct.id AS contract_id, ct.coach_id,
        pay.amount_yen - pay.refunded_yen AS gross,
        CASE WHEN cp.reduced_fee_until IS NOT NULL AND cp.reduced_fee_until > ct.starts_at
             THEN 100 - sqlc.arg(reduced_fee_percent)::int ELSE 100 - sqlc.arg(fee_percent)::int END AS share
    FROM coaching_contracts ct
    JOIN coaching_payments pay ON pay.contract_id = ct.id
    LEFT JOIN coach_profiles cp ON cp.profile_id = ct.coach_id
    WHERE ct.status IN ('ended', 'refunded')
      AND pay.status IN ('paid', 'partially_refunded')
      AND pay.refund_reason IS DISTINCT FROM 'not_started'
      AND NOT EXISTS (SELECT 1 FROM coach_earnings e WHERE e.contract_id = ct.id)
    ORDER BY ct.id, pay.paid_at
) x
WHERE x.gross > 0
LIMIT 100
ON CONFLICT DO NOTHING
RETURNING *;

-- name: LockCoachPayouts :exec
-- 送金をまとめる処理が同時に2つ動かないようにする。
SELECT pg_advisory_xact_lock(hashtext('coach_payouts'));

-- name: ListPayableCoachEarnings :many
-- 今月（日本時間）より前の、まだ送金していない報酬。受け取りの準備ができたコーチの分だけ。
SELECT e.coach_id::uuid AS coach_id, sum(e.amount_yen)::int AS amount_yen
FROM coach_earnings e
JOIN coach_profiles cp ON cp.profile_id = e.coach_id
WHERE e.payout_id IS NULL
  AND e.created_at < date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo') AT TIME ZONE 'Asia/Tokyo'
  AND cp.payouts_enabled AND cp.stripe_account_id IS NOT NULL
GROUP BY e.coach_id;

-- name: UpsertCoachPayout :one
-- 前の月の分の送金。同じ月の送金がまだ済んでいなければ、そこに足す。
INSERT INTO coach_payouts (id, coach_id, month, amount_yen)
VALUES (sqlc.arg(id), sqlc.arg(coach_id),
        (date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo') - interval '1 month')::date, sqlc.arg(amount_yen))
ON CONFLICT (coach_id, month) DO UPDATE
SET amount_yen = coach_payouts.amount_yen + EXCLUDED.amount_yen,
    status = 'pending',
    failure = NULL
WHERE coach_payouts.status <> 'paid'
RETURNING *;

-- name: AttachEarningsToPayout :execrows
UPDATE coach_earnings
SET payout_id = sqlc.arg(payout_id)
WHERE coach_id = sqlc.arg(coach_id) AND payout_id IS NULL
  AND created_at < date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo') AT TIME ZONE 'Asia/Tokyo';

-- name: ListUnpaidCoachPayouts :many
-- まだ送金していない（前に失敗した）送金と、送り先のアカウント。
SELECT p.*, cp.stripe_account_id::text AS stripe_account_id
FROM coach_payouts p
JOIN coach_profiles cp ON cp.profile_id = p.coach_id
WHERE p.status IN ('pending', 'failed') AND cp.payouts_enabled AND cp.stripe_account_id IS NOT NULL
ORDER BY p.created_at
LIMIT 100;

-- name: MarkCoachPayoutPaid :one
UPDATE coach_payouts
SET status = 'paid', stripe_transfer_id = sqlc.arg(stripe_transfer_id), failure = NULL, paid_at = now()
WHERE id = sqlc.arg(id) AND status <> 'paid'
RETURNING *;

-- name: MarkCoachPayoutFailed :exec
UPDATE coach_payouts
SET status = 'failed', failure = sqlc.arg(failure)
WHERE id = sqlc.arg(id) AND status <> 'paid';

-- name: ListCoachPayouts :many
SELECT * FROM coach_payouts
WHERE coach_id = $1
ORDER BY month DESC
LIMIT 24;

-- name: ListRecentCoachEarnings :many
SELECT * FROM coach_earnings
WHERE coach_id = $1
ORDER BY created_at DESC
LIMIT 30;

-- name: GetCoachEarningsSummary :one
-- 今月（日本時間）の報酬と、まだ送金していない報酬。
SELECT
    COALESCE(sum(amount_yen) FILTER (
        WHERE created_at >= date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo') AT TIME ZONE 'Asia/Tokyo'), 0)::int AS this_month_yen,
    COALESCE(sum(amount_yen) FILTER (WHERE payout_id IS NULL), 0)::int AS unpaid_yen
FROM coach_earnings
WHERE coach_id = $1;

-- name: AdminListPayoutCoaches :many
-- 運営の送金の画面に出す、承認済みのコーチ。
SELECT cp.profile_id, p.nickname, cp.tier, (cp.stripe_account_id IS NOT NULL)::boolean AS stripe_linked,
       cp.payouts_enabled, cp.reduced_fee_until,
       COALESCE((SELECT sum(e.amount_yen) FROM coach_earnings e WHERE e.coach_id = cp.profile_id AND e.payout_id IS NULL), 0)::int AS unpaid_yen
FROM coach_profiles cp
JOIN profiles p ON p.id = cp.profile_id
WHERE cp.status = 'approved'
ORDER BY unpaid_yen DESC, cp.reviewed_at
LIMIT 200;

-- name: AdminListCoachPayouts :many
SELECT cpo.*, p.nickname
FROM coach_payouts cpo
LEFT JOIN profiles p ON p.id = cpo.coach_id
ORDER BY cpo.created_at DESC
LIMIT 100;
