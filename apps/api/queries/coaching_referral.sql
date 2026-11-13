-- name: IssueCoachingReferral :exec
SELECT coaching_issue_referral($1::uuid, 'purchase');

-- name: CountCoachingReferral :one
SELECT COALESCE(coaching_count_referral(sqlc.arg(buyer)::uuid, sqlc.arg(code)::text,
    sqlc.narg(ticket_id)::uuid, sqlc.narg(payment_id)::uuid)::text, '')::text AS recipient;

-- name: GetUsableCoachingReferral :one
SELECT code FROM coaching_referral_codes WHERE code = $1 AND owner_id <> sqlc.arg(buyer)::uuid;

-- name: GetCoachingReferralProgress :one
SELECT c.code,
    (SELECT count(*)::int FROM coaching_referral_counts a
     WHERE a.referrer_id = c.owner_id AND a.canceled_at IS NULL) AS friend_count,
    r.granted_at, l.expires_at
FROM coaching_referral_codes c
LEFT JOIN coaching_referral_rewards r ON r.referrer_id = c.owner_id AND r.canceled_at IS NULL
LEFT JOIN coaching_ticket_lots l ON l.id = r.lot_id
WHERE c.owner_id = $1;

-- name: ReferralRewardHasRefund :one
SELECT EXISTS(SELECT FROM coaching_referral_rewards
    WHERE lot_id = $1 AND qualifying_refund_at IS NOT NULL)::boolean;

-- name: CancelReleasedReferralReward :exec
UPDATE coaching_referral_rewards SET canceled_at = now()
WHERE lot_id = $1 AND qualifying_refund_at IS NOT NULL AND canceled_at IS NULL
  AND EXISTS (SELECT FROM coaching_ticket_lots l WHERE l.id = coaching_referral_rewards.lot_id AND l.reserved = 0 AND l.consumed = 0);

-- name: AdminCoachingReferralOverview :many
SELECT c.code, p.id AS owner_id, p.nickname,
    ARRAY(SELECT a.buyer_id::text FROM coaching_referral_counts a
          WHERE a.referrer_id = c.owner_id AND a.canceled_at IS NULL AND a.buyer_id IS NOT NULL
          ORDER BY a.counted_at)::text[] AS buyers,
    r.granted_at, r.canceled_at, r.qualifying_refund_at
FROM coaching_referral_codes c
JOIN profiles p ON p.id = c.owner_id
LEFT JOIN coaching_referral_rewards r ON r.referrer_id = c.owner_id
WHERE (sqlc.arg(code)::text = '' AND r.lot_id IS NOT NULL) OR c.code = sqlc.arg(code)::text
ORDER BY r.granted_at DESC NULLS LAST, c.code
LIMIT 200;
