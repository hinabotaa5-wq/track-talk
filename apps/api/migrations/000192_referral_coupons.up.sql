-- 紹介特典を「無料チケット1枚」から「¥500 クーポン」に変える（本体 #1031）。
-- クーポンは運営が負担し、コーチ報酬は割引前の価格 × 70% で計算する。
-- 細則は無料チケットに合わせる：付与から6か月有効／付与された本人だけが使える／現金への払戻しはできない。
-- 1回の申込に使えるクーポンは1枚まで。依頼が不成立で終わったら、チケットと同じ時点・条件で未使用に戻す。
-- 発行済みの特典チケット（coaching_ticket_lots の kind='referral_reward'）と、その特典行（lot_id）はそのまま残し、
-- 従来どおり有効期限まで使える（この移行ではチケットのロット・台帳を変更・削除しない）。

-- クーポン1枚が1行。状態：unused（未使用）／reserved（申込で確保中）／used（使用済）／expired（失効）／
-- canceled（紹介の対象購入が返金されて取り消し）。
CREATE TABLE coaching_coupons (
    id uuid PRIMARY KEY,
    owner_id uuid REFERENCES profiles(id) ON DELETE SET NULL,
    -- referral：紹介特典／reissue：運営者・コーチ都合で終わった依頼で失効に回った分の出し直し（チケットの #797 と同じ）。
    source text NOT NULL CHECK (source IN ('referral', 'reissue')),
    amount_yen integer NOT NULL CHECK (amount_yen > 0),
    status text NOT NULL DEFAULT 'unused' CHECK (status IN ('unused', 'reserved', 'used', 'expired', 'canceled')),
    -- 確保中・使用済の依頼。依頼が消えたら NULL（確保中なら tick が戻す）。
    request_id uuid REFERENCES review_requests(id) ON DELETE SET NULL,
    granted_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    reserved_at timestamptz,
    used_at timestamptz,
    ended_at timestamptz,
    CHECK (status <> 'reserved' OR reserved_at IS NOT NULL),
    CHECK (status <> 'used' OR used_at IS NOT NULL),
    CHECK (status NOT IN ('expired', 'canceled') OR ended_at IS NOT NULL)
);
CREATE INDEX coaching_coupons_owner_idx ON coaching_coupons(owner_id, status, expires_at);
CREATE INDEX coaching_coupons_reserved_idx ON coaching_coupons(request_id) WHERE status = 'reserved';
-- 1回の申込に使えるクーポンは1枚まで。
CREATE UNIQUE INDEX coaching_coupons_one_per_request ON coaching_coupons(request_id) WHERE status IN ('reserved', 'used');
ALTER TABLE coaching_coupons ENABLE ROW LEVEL SECURITY;

-- 依頼に適用したクーポンの割引額（運営負担分）。選手の支払額は base_price_yen - coupon_discount_yen。
ALTER TABLE review_requests
    ADD COLUMN coupon_discount_yen integer NOT NULL DEFAULT 0,
    ADD CONSTRAINT review_requests_coupon_discount CHECK (coupon_discount_yen >= 0 AND coupon_discount_yen <= base_price_yen),
    -- チケットとクーポンは併用できない。
    ADD CONSTRAINT review_requests_coupon_not_tickets CHECK (NOT (use_tickets AND coupon_discount_yen > 0));

-- 報酬レコードに運営負担分を残す。gross_yen は割引前の価格、選手の実支払額は gross_yen - platform_subsidy_yen。
ALTER TABLE coach_earnings
    ADD COLUMN platform_subsidy_yen integer NOT NULL DEFAULT 0,
    ADD CONSTRAINT coach_earnings_platform_subsidy CHECK (platform_subsidy_yen >= 0 AND platform_subsidy_yen <= gross_yen);

-- 特典行は、発行済みのチケット（lot_id）か、新しいクーポン（coupon_id）のどちらか1つを指す。
ALTER TABLE coaching_referral_rewards DROP CONSTRAINT coaching_referral_rewards_pkey;
ALTER TABLE coaching_referral_rewards
    ALTER COLUMN lot_id DROP NOT NULL,
    ADD CONSTRAINT coaching_referral_rewards_lot_id_key UNIQUE (lot_id),
    ADD COLUMN coupon_id uuid UNIQUE REFERENCES coaching_coupons(id),
    ADD CONSTRAINT coaching_referral_reward_target CHECK (num_nonnulls(lot_id, coupon_id) = 1);

-- 周の達成で付与する特典を ¥500 クーポンにする（判定・ID の終了は 000170 と同じ）。
CREATE OR REPLACE FUNCTION coaching_grant_referral(cycle text) RETURNS uuid LANGUAGE plpgsql AS $$
DECLARE
    c coaching_referral_codes;
    qualified uuid[];
    reward uuid;
BEGIN
    SELECT * INTO c FROM coaching_referral_codes WHERE code = cycle;
    IF c.code IS NULL OR c.closed_at IS NOT NULL THEN
        RETURN NULL;
    END IF;
    SELECT array_agg(id ORDER BY counted_at, id) INTO qualified
        FROM coaching_referral_counts WHERE referral_code = cycle AND canceled_at IS NULL;
    IF COALESCE(cardinality(qualified), 0) < 2 THEN
        RETURN NULL;
    END IF;
    -- 起点購入の支払いが確定していなければ付与しない（パックは支払い完了、単発は capture 済み）。
    IF c.origin_ticket_purchase_id IS NOT NULL AND NOT EXISTS (
        SELECT FROM coaching_ticket_purchases WHERE id = c.origin_ticket_purchase_id AND paid_at IS NOT NULL
    ) OR c.origin_review_payment_id IS NOT NULL AND NOT EXISTS (
        SELECT FROM review_payments WHERE id = c.origin_review_payment_id AND captured_at IS NOT NULL
    ) THEN
        RETURN NULL;
    END IF;
    reward := gen_random_uuid();
    INSERT INTO coaching_coupons(id, owner_id, source, amount_yen, expires_at)
    VALUES (reward, c.owner_id, 'referral', 500, now() + interval '6 months');
    INSERT INTO coaching_referral_rewards(coupon_id, referrer_id, first_count_id, second_count_id, referral_code)
    VALUES (reward, c.owner_id, qualified[1], qualified[2], cycle);
    UPDATE coaching_referral_codes SET closed_at = now() WHERE code = cycle;
    RETURN c.owner_id;
END
$$;

-- カウント1件を取り消し、そのカウントで付与された未使用の特典を取り消す（000170 にクーポンの分岐を足した）。
-- 発行済みのチケットの特典は 000170 と同じ扱い。クーポンは未使用なら取り消し、確保中なら申込を尊重して
-- 戻すときに取り消す（API が qualifying_refund_at を見る）。使用済みはそのまま。
CREATE OR REPLACE FUNCTION coaching_cancel_referral_count(count_id uuid) RETURNS void LANGUAGE plpgsql AS $$
DECLARE
    contribution coaching_referral_counts;
    reward coaching_referral_rewards;
    lot coaching_ticket_lots;
    coupon coaching_coupons;
BEGIN
    SELECT * INTO contribution FROM coaching_referral_counts WHERE id = count_id;
    IF contribution.id IS NULL THEN
        RETURN;
    END IF;
    PERFORM 1 FROM coaching_referral_codes WHERE code = contribution.referral_code FOR NO KEY UPDATE;
    UPDATE coaching_referral_counts SET canceled_at = now() WHERE id = contribution.id AND canceled_at IS NULL;
    IF NOT FOUND THEN
        RETURN;
    END IF;

    SELECT * INTO reward FROM coaching_referral_rewards
        WHERE canceled_at IS NULL AND contribution.id IN (first_count_id, second_count_id);
    IF NOT FOUND THEN
        RETURN;
    END IF;
    IF reward.coupon_id IS NOT NULL THEN
        SELECT * INTO coupon FROM coaching_coupons WHERE id = reward.coupon_id FOR UPDATE;
        UPDATE coaching_referral_rewards SET qualifying_refund_at = now(),
            canceled_at = CASE WHEN coupon.status IN ('unused', 'expired') THEN now() END
        WHERE coupon_id = coupon.id;
        IF coupon.status = 'unused' THEN
            UPDATE coaching_coupons SET status = 'canceled', ended_at = now() WHERE id = coupon.id;
        END IF;
        RETURN;
    END IF;
    SELECT * INTO lot FROM coaching_ticket_lots WHERE id = reward.lot_id FOR UPDATE;
    UPDATE coaching_referral_rewards SET qualifying_refund_at = now(),
        canceled_at = CASE WHEN lot.consumed = 0 AND lot.reserved = 0 THEN now() END
    WHERE lot_id = lot.id;
    -- Honor an existing reservation. On release, the API checks this
    -- recorded refund and discards the ticket instead of restoring it.
    IF lot.available > 0 THEN
        UPDATE coaching_ticket_lots SET available = 0, expired = expired + lot.available
        WHERE id = lot.id;
        INSERT INTO coaching_ticket_ledger(id, lot_id, operation_key, kind, available_delta, expired_delta)
        VALUES (gen_random_uuid(), lot.id, 'referral-cancel-' || lot.id, 'expire', -lot.available, lot.available);
    END IF;
END
$$;
