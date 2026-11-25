-- 紹介特典の ¥500 クーポン（本体 #1031）。

-- name: CouponBalance :one
-- 使える（未使用で期限内の）クーポンの枚数と、いちばん早い有効期限。期限切れは tick を待たずに外す。
SELECT count(*)::bigint AS available, COALESCE(min(expires_at), now())::timestamptz AS next_expires_at,
    COALESCE(min(amount_yen), 0)::integer AS amount_yen
FROM coaching_coupons WHERE owner_id = $1 AND status = 'unused' AND expires_at > now();

-- name: LockUsableCoupon :one
-- 申込に使うクーポン1枚。指定があればその1枚（本人の・未使用・期限内のときだけ）、なければ期限の早いもの。
SELECT * FROM coaching_coupons
WHERE owner_id = sqlc.arg(owner_id)::uuid AND status = 'unused' AND expires_at > now()
  AND (sqlc.narg(coupon_id)::uuid IS NULL OR id = sqlc.narg(coupon_id)::uuid)
ORDER BY expires_at, granted_at, id
LIMIT 1 FOR UPDATE;

-- name: ReserveCoupon :exec
UPDATE coaching_coupons SET status = 'reserved', request_id = $2, reserved_at = now()
WHERE id = $1 AND status = 'unused';

-- name: GetReservedCouponForReview :one
SELECT * FROM coaching_coupons WHERE request_id = $1 AND status = 'reserved' FOR UPDATE;

-- name: GetCouponForReview :one
-- 依頼に使った（確保中・使用済の）クーポン。
SELECT * FROM coaching_coupons WHERE request_id = $1 AND status IN ('reserved', 'used');

-- name: LockCoupon :one
SELECT * FROM coaching_coupons WHERE id = $1 FOR UPDATE;

-- name: UseCoupon :exec
UPDATE coaching_coupons SET status = 'used', used_at = now() WHERE id = $1 AND status = 'reserved';

-- name: ReleaseCoupon :exec
-- 不成立で終わった依頼のクーポンを未使用に戻す。
UPDATE coaching_coupons SET status = 'unused', request_id = NULL, reserved_at = NULL
WHERE id = $1 AND status = 'reserved';

-- name: EndCoupon :exec
-- 失効（expired）か取り消し（canceled）で終える。
UPDATE coaching_coupons SET status = sqlc.arg(status)::text, ended_at = now(), request_id = NULL, reserved_at = NULL
WHERE id = sqlc.arg(id)::uuid AND status IN ('unused', 'reserved');

-- name: ListCouponsToRelease :many
-- 確保中のまま依頼が不成立で終わった（期限切れ・取り消しで expired、または依頼が消えた）クーポン。
SELECT c.id, c.request_id FROM coaching_coupons c LEFT JOIN review_requests r ON r.id = c.request_id
WHERE c.status = 'reserved' AND (r.id IS NULL OR r.status = 'expired')
ORDER BY c.reserved_at LIMIT 100;

-- name: ExpireUnusedCoupons :execrows
-- 有効期限を過ぎた未使用のクーポンを失効させる（延長・救済はしない）。
UPDATE coaching_coupons SET status = 'expired', ended_at = now()
WHERE status = 'unused' AND expires_at <= now();

-- name: CouponRewardHasRefund :one
-- 紹介の対象購入が返金・チャージバックで取り消された特典のクーポンか。
SELECT EXISTS(SELECT FROM coaching_referral_rewards
    WHERE coupon_id = $1 AND qualifying_refund_at IS NOT NULL)::boolean;

-- name: CancelCouponReward :exec
UPDATE coaching_referral_rewards SET canceled_at = now()
WHERE coupon_id = $1 AND qualifying_refund_at IS NOT NULL AND canceled_at IS NULL;

-- name: CreateReissueCoupon :exec
-- 運営者・コーチ都合で終わった依頼で失効に回ったクーポンの出し直し（チケットの #797 と同じ）。
-- 旧クーポンの期限を延ばすのではなく、出し直した日から6か月有効の新しい1枚を付与する。特典行は作らない。
INSERT INTO coaching_coupons (id, owner_id, source, amount_yen, expires_at)
VALUES ($1, $2, 'reissue', $3, now() + interval '6 months');

-- name: RecordCouponReviewEarnings :exec
-- 選手の支払いが0円になった（価格の全額をクーポンで割り引いた）添削の報酬。割引前の価格 × 割合で記録する。
-- 支払いがある添削は InsertReviewEarnings（capture 後）で記録する。
INSERT INTO coach_earnings (id, coach_id, source, review_request_id, gross_yen, share_percent, amount_yen, confirmed_at, platform_subsidy_yen)
SELECT gen_random_uuid(), r.coach_id, 'review', r.id, r.base_price_yen, sqlc.arg(share_percent)::smallint,
       r.base_price_yen * sqlc.arg(share_percent)::int / 100, r.returned_at, r.coupon_discount_yen
FROM review_requests r
WHERE r.id = sqlc.arg(request_id)::uuid AND NOT r.use_tickets AND r.coupon_discount_yen >= r.base_price_yen
  AND r.returned_at IS NOT NULL AND r.coach_id IS NOT NULL
ON CONFLICT (review_request_id) DO NOTHING;
