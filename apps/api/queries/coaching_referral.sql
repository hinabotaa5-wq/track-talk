-- name: RecordCoachingReferral :one
-- 確定した対象購入を記録する（入力IDで数え、有効カウントが紐付かなければ起点としてIDを発行）。
SELECT COALESCE(coaching_record_referral(sqlc.arg(buyer)::uuid, sqlc.narg(code)::text,
    sqlc.narg(ticket_id)::uuid, sqlc.narg(payment_id)::uuid)::text, '')::text AS recipient;

-- name: IssueCoachingReferralOrigin :exec
-- 単発カードの仮押さえ時点の起点購入として紹介IDを発行する（有効なIDがあれば何もしない）。
SELECT coaching_issue_referral_code(sqlc.arg(owner)::uuid, 'purchase', NULL, sqlc.narg(payment_id)::uuid);

-- name: CoachingReferralAlreadyCounted :one
-- 入力された紹介IDの紹介者に、この購入者がすでに有効カウントされているか。
SELECT EXISTS (SELECT FROM coaching_referral_codes c JOIN coaching_referral_counts a ON a.referrer_id = c.owner_id
    WHERE c.code = $1 AND a.buyer_id = sqlc.arg(buyer)::uuid AND a.canceled_at IS NULL)::boolean;

-- name: CancelDisputedCoachingReferral :exec
SELECT coaching_cancel_referral_for_payment_intent($1::text);

-- name: GetUsableCoachingReferral :one
-- 使える紹介ID：他人の・終了していないIDで、その紹介者に有効カウント済みの購入者ではない。
SELECT c.code FROM coaching_referral_codes c
WHERE c.code = $1 AND c.owner_id <> sqlc.arg(buyer)::uuid AND c.closed_at IS NULL
  AND NOT EXISTS (SELECT FROM coaching_referral_counts a
                  WHERE a.referrer_id = c.owner_id AND a.buyer_id = sqlc.arg(buyer)::uuid AND a.canceled_at IS NULL);

-- name: GetCoachingReferralProgress :one
-- 有効な周があればそれを、なければ直近に終了した周を返す。進捗は周に所属する。
SELECT c.code, (c.closed_at IS NULL)::boolean AS active,
    (SELECT count(*)::int FROM coaching_referral_counts a
     WHERE a.referral_code = c.code AND a.canceled_at IS NULL) AS friend_count,
    r.granted_at, l.expires_at
FROM coaching_referral_codes c
LEFT JOIN coaching_referral_rewards r ON r.referral_code = c.code AND r.canceled_at IS NULL
LEFT JOIN coaching_ticket_lots l ON l.id = r.lot_id
WHERE c.owner_id = $1
ORDER BY c.closed_at IS NULL DESC, c.issued_at DESC
LIMIT 1;

-- name: ReferralRewardHasRefund :one
SELECT EXISTS(SELECT FROM coaching_referral_rewards
    WHERE lot_id = $1 AND qualifying_refund_at IS NOT NULL)::boolean;

-- name: CancelReleasedReferralReward :exec
UPDATE coaching_referral_rewards SET canceled_at = now()
WHERE lot_id = $1 AND qualifying_refund_at IS NOT NULL AND canceled_at IS NULL
  AND EXISTS (SELECT FROM coaching_ticket_lots l WHERE l.id = coaching_referral_rewards.lot_id AND l.reserved = 0 AND l.consumed = 0);

-- name: AdminCoachingReferralOverview :many
SELECT c.code, p.id AS owner_id, p.nickname, c.issued_at, c.closed_at,
    ARRAY(SELECT a.buyer_id::text FROM coaching_referral_counts a
          WHERE a.referral_code = c.code AND a.canceled_at IS NULL AND a.buyer_id IS NOT NULL
          ORDER BY a.counted_at)::text[] AS buyers,
    r.granted_at, r.canceled_at, r.qualifying_refund_at
FROM coaching_referral_codes c
JOIN profiles p ON p.id = c.owner_id
LEFT JOIN coaching_referral_rewards r ON r.lot_id = (
    SELECT x.lot_id FROM coaching_referral_rewards x WHERE x.referral_code = c.code
    ORDER BY x.canceled_at IS NULL DESC, x.granted_at DESC LIMIT 1)
WHERE (sqlc.arg(code)::text = '' AND r.lot_id IS NOT NULL) OR c.code = sqlc.arg(code)::text
ORDER BY r.granted_at DESC NULLS LAST, c.code
LIMIT 200;
