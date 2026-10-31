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

-- name: SetCoachPayoutsEnabledByAccount :one
-- Stripe から口座の状態が変わったと届いた。登録が済んだ時も、あとから送金を受け取れなくなった時も反映する。
UPDATE coach_profiles
SET payouts_enabled = sqlc.arg(payouts_enabled),
    updated_at = now()
WHERE stripe_account_id = sqlc.arg(stripe_account_id)
RETURNING *;

-- name: InsertReviewEarnings :many
-- 返却して代金が確定した添削の、コーチの取り分を記録する。チケットで払った添削は RecordTicketReviewEarnings で記録する。
INSERT INTO coach_earnings (id, coach_id, source, review_request_id, gross_yen, share_percent, amount_yen)
SELECT gen_random_uuid(), r.coach_id, 'review', r.id, pay.amount_yen, sqlc.arg(share_percent)::smallint,
       pay.amount_yen * sqlc.arg(share_percent)::int / 100
FROM review_payments pay
JOIN review_requests r ON r.id = pay.request_id
WHERE pay.status = 'captured' AND r.returned_at IS NOT NULL AND r.coach_id IS NOT NULL
  AND NOT r.use_tickets
  AND NOT EXISTS (SELECT 1 FROM coach_earnings e WHERE e.review_request_id = r.id)
LIMIT 100
ON CONFLICT DO NOTHING
RETURNING *;

-- name: InsertPersonalEarnings :many
-- 期間が終わった（途中で返金して閉じた）パーソナル指導の、手数料を引いたコーチの取り分を記録する。
INSERT INTO coach_earnings (id, coach_id, source, contract_id, gross_yen, share_percent, amount_yen)
SELECT gen_random_uuid(), x.coach_id, 'personal', x.contract_id, x.gross, x.share, x.gross * x.share / 100
FROM (
    SELECT DISTINCT ON (ct.id)
        ct.id AS contract_id, ct.coach_id,
        pay.amount_yen - pay.refunded_yen AS gross,
        100 - sqlc.arg(fee_percent)::int AS share
    FROM coaching_contracts ct
    JOIN coaching_payments pay ON pay.contract_id = ct.id
    WHERE ct.status IN ('ended', 'refunded') AND (ct.fulfillment_version=0 OR ct.fulfillment_state='settled')
      AND pay.status IN ('paid', 'partially_refunded')
      AND pay.refund_reason IS DISTINCT FROM 'not_started'
      AND NOT EXISTS (SELECT 1 FROM coach_earnings e WHERE e.contract_id = ct.id)
    ORDER BY ct.id, pay.paid_at
) x
-- 取り分が0円になる行（1円だけ残った返金など）は記録しない。amount_yen > 0 の制約で全体が失敗するため。
WHERE x.gross * x.share / 100 > 0
LIMIT 100
ON CONFLICT DO NOTHING
RETURNING *;

-- name: LockCoachPayouts :exec
-- 送金をまとめる処理が同時に2つ動かないようにする。
SELECT pg_advisory_xact_lock(hashtext('coach_payouts'));

-- name: ListPayableCoachEarnings :many
-- 今月（日本時間）より前の、まだ送金していない報酬。受け取りの準備ができたコーチの分だけ。
-- 送金ごとに Stripe の費用がかかるので、合計が min_yen に届かない分は翌月に繰り越す。
-- 繰り越しは1回まで。前の前の月より前の報酬が残っていれば、届かなくても送る。
-- 活動を終えた（承認済みでなくなった）コーチにも、届かなくても残りを送る。
SELECT e.coach_id::uuid AS coach_id, sum(e.amount_yen - COALESCE((SELECT sum(a.amount_yen) FROM coach_refund_applications a WHERE a.earning_id=e.id),0))::int AS amount_yen
FROM coach_earnings e
JOIN coach_profiles cp ON cp.profile_id = e.coach_id
WHERE e.payout_id IS NULL
  AND e.amount_yen > COALESCE((SELECT sum(a.amount_yen) FROM coach_refund_applications a WHERE a.earning_id=e.id),0)
  AND NOT EXISTS (SELECT 1 FROM captured_payment_refunds rf WHERE rf.status='pending' AND ((rf.source='review' AND e.source='review' AND rf.payment_id IN (SELECT payment_id FROM review_payment_earning_links WHERE earning_id=e.id)) OR (rf.source='consultation' AND rf.payment_id=e.consultation_id)))
  AND NOT EXISTS (SELECT 1 FROM captured_refund_earnings f WHERE f.earning_id=e.id AND f.excluded)
  AND e.created_at < date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo') AT TIME ZONE 'Asia/Tokyo'
  AND cp.payouts_enabled AND cp.stripe_account_id IS NOT NULL
GROUP BY e.coach_id, cp.status
HAVING sum(e.amount_yen - COALESCE((SELECT sum(a.amount_yen) FROM coach_refund_applications a WHERE a.earning_id=e.id),0)) >= sqlc.arg(min_yen)::int OR cp.status <> 'approved'
    OR min(e.created_at) < (date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo') - interval '1 month') AT TIME ZONE 'Asia/Tokyo';

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
UPDATE coach_earnings e
SET payout_id = sqlc.arg(payout_id)
WHERE coach_id = sqlc.arg(coach_id) AND payout_id IS NULL
  AND NOT EXISTS (SELECT 1 FROM captured_payment_refunds rf WHERE rf.status='pending' AND ((rf.source='review' AND e.source='review' AND rf.payment_id IN (SELECT payment_id FROM review_payment_earning_links WHERE earning_id=e.id)) OR (rf.source='consultation' AND rf.payment_id=e.consultation_id)))
  AND NOT EXISTS (SELECT 1 FROM captured_refund_earnings f WHERE f.earning_id=e.id AND f.excluded)
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
SELECT
 COALESCE(sum(e.amount_yen) FILTER (WHERE e.created_at >= date_trunc('month',now() AT TIME ZONE 'Asia/Tokyo') AT TIME ZONE 'Asia/Tokyo'),0)::int AS this_month_yen,
 COALESCE(sum(e.amount_yen - COALESCE((SELECT sum(a.amount_yen) FROM coach_refund_applications a WHERE a.earning_id=e.id),0)) FILTER (WHERE e.payout_id IS NULL),0)::int AS unpaid_yen
FROM coach_earnings e WHERE e.coach_id=$1 AND NOT EXISTS(SELECT 1 FROM captured_refund_earnings f WHERE f.earning_id=e.id AND f.excluded);

-- name: AdminListPayoutCoaches :many
-- 運営の送金の画面に出す、承認済みのコーチ。
SELECT cp.profile_id, p.nickname, cp.tier, (cp.stripe_account_id IS NOT NULL)::boolean AS stripe_linked,
       cp.payouts_enabled,
       COALESCE((SELECT sum(e.amount_yen - COALESCE((SELECT sum(a.amount_yen) FROM coach_refund_applications a WHERE a.earning_id=e.id),0)) FROM coach_earnings e WHERE e.coach_id = cp.profile_id AND e.payout_id IS NULL AND NOT EXISTS(SELECT 1 FROM captured_refund_earnings f WHERE f.earning_id=e.id AND f.excluded)), 0)::int AS unpaid_yen
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

-- name: GetContractEarning :one
SELECT * FROM coach_earnings WHERE contract_id = $1;

-- name: DeleteUnpaidContractEarning :exec
-- 期間が終わってから全額を返金した指導の、まだ送金に含めていない報酬を消す。
DELETE FROM coach_earnings WHERE contract_id = $1 AND payout_id IS NULL;

-- name: CoachStripeAccountExists :one
SELECT EXISTS (SELECT 1 FROM coach_profiles WHERE stripe_account_id = sqlc.arg(stripe_account_id)::text);
