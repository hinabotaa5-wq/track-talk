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
-- 有効な周があればそれを、なければ終了から7日未満の周のうち直近の周を返す。進捗は周に所属する。
SELECT c.code, (c.closed_at IS NULL)::boolean AS active,
    (SELECT count(*)::int FROM coaching_referral_counts a
     WHERE a.referral_code = c.code AND a.canceled_at IS NULL) AS friend_count,
    r.granted_at, COALESCE(l.expires_at, cp.expires_at, 'epoch'::timestamptz)::timestamptz AS expires_at
FROM coaching_referral_codes c
LEFT JOIN coaching_referral_rewards r ON r.referral_code = c.code AND r.canceled_at IS NULL
LEFT JOIN coaching_ticket_lots l ON l.id = r.lot_id
LEFT JOIN coaching_referral_coupons cp ON cp.id = r.coupon_id
WHERE c.owner_id = $1
  AND (c.closed_at IS NULL OR c.closed_at > now() - interval '7 days')
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
    r.granted_at, r.canceled_at, r.qualifying_refund_at,
    COALESCE(l.expires_at, cp.expires_at, 'epoch'::timestamptz)::timestamptz AS expires_at
FROM coaching_referral_codes c
JOIN profiles p ON p.id = c.owner_id
LEFT JOIN coaching_referral_rewards r ON r.referral_code = c.code AND r.canceled_at IS NULL
LEFT JOIN coaching_ticket_lots l ON l.id = r.lot_id
LEFT JOIN coaching_referral_coupons cp ON cp.id = r.coupon_id
WHERE (sqlc.arg(code)::text = '' AND (r.lot_id IS NOT NULL OR r.coupon_id IS NOT NULL)) OR c.code = sqlc.arg(code)::text
ORDER BY r.granted_at DESC NULLS LAST, c.code
LIMIT 200;

-- name: ReferralCouponBalance :one
SELECT count(*)::int AS count,
    COALESCE(min(expires_at), 'epoch'::timestamptz)::timestamptz AS earliest_expires_at
FROM coaching_referral_coupons
WHERE owner_id = $1 AND used_at IS NULL AND canceled_at IS NULL AND expires_at > now();

-- name: LockAvailableReferralCoupon :one
SELECT id FROM coaching_referral_coupons
WHERE owner_id = $1 AND used_at IS NULL AND canceled_at IS NULL AND expires_at > now()
ORDER BY expires_at, id LIMIT 1 FOR UPDATE;

-- name: UseReferralCoupon :exec
UPDATE coaching_referral_coupons SET used_at = now(), used_reservation_id = $2 WHERE id = $1;

-- name: ReviewUsedReferralCoupon :one
SELECT EXISTS (
    SELECT FROM coaching_ticket_reservations t
    JOIN coaching_referral_coupons cp ON cp.used_reservation_id = t.id
    WHERE t.request_id = $1
)::boolean;

-- name: LockReferralCodeForCouponRelease :exec
SELECT c.code FROM coaching_referral_codes c
JOIN coaching_referral_rewards r ON r.referral_code = c.code
JOIN coaching_referral_coupons cp ON cp.id = r.coupon_id
WHERE cp.used_reservation_id = $1 FOR NO KEY UPDATE OF c;

-- name: LockReferralCouponByReservation :one
SELECT * FROM coaching_referral_coupons WHERE used_reservation_id = $1 FOR UPDATE;

-- name: ReferralCouponHasRefund :one
SELECT EXISTS (SELECT FROM coaching_referral_rewards
    WHERE coupon_id = $1 AND qualifying_refund_at IS NOT NULL)::boolean;

-- name: ReleaseReferralCoupon :exec
UPDATE coaching_referral_coupons SET used_at = NULL, used_reservation_id = NULL WHERE id = $1;

-- name: CancelReleasedReferralCoupon :exec
WITH canceled AS (
    UPDATE coaching_referral_coupons cp SET canceled_at = now() WHERE cp.id = $1 RETURNING cp.id
)
UPDATE coaching_referral_rewards SET canceled_at = now()
WHERE coupon_id IN (SELECT id FROM canceled) AND canceled_at IS NULL;

-- name: ReissueReferralCoupon :exec
INSERT INTO coaching_referral_coupons(owner_id, expires_at) VALUES ($1, now() + interval '6 months');
