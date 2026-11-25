-- 注意：この down は、000192 で追加したクーポンを消す。付与・使用されたクーポンと、クーポンを指す紹介特典の行、
-- 報酬レコードの運営負担分（platform_subsidy_yen）、依頼のクーポン割引額（coupon_discount_yen）は失われ、戻せない。
-- 紹介特典の付与と取り消しは 000170 の定義（特典チケットのロット）に戻す。

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
    INSERT INTO coaching_ticket_lots(id, owner_id, quantity, available, expires_at, kind)
    VALUES (reward, c.owner_id, 1, 1, now() + interval '6 months', 'referral_reward');
    INSERT INTO coaching_ticket_ledger(id, lot_id, operation_key, kind, available_delta)
    VALUES (gen_random_uuid(), reward, 'referral-grant-' || reward, 'referral_grant', 1);
    INSERT INTO coaching_referral_rewards(lot_id, referrer_id, first_count_id, second_count_id, referral_code)
    VALUES (reward, c.owner_id, qualified[1], qualified[2], cycle);
    UPDATE coaching_referral_codes SET closed_at = now() WHERE code = cycle;
    RETURN c.owner_id;
END
$$;

CREATE OR REPLACE FUNCTION coaching_cancel_referral_count(count_id uuid) RETURNS void LANGUAGE plpgsql AS $$
DECLARE
    contribution coaching_referral_counts;
    reward coaching_referral_rewards;
    lot coaching_ticket_lots;
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

DELETE FROM coaching_referral_rewards WHERE coupon_id IS NOT NULL;
ALTER TABLE coaching_referral_rewards
    DROP CONSTRAINT coaching_referral_reward_target,
    DROP COLUMN coupon_id,
    DROP CONSTRAINT coaching_referral_rewards_lot_id_key,
    ALTER COLUMN lot_id SET NOT NULL,
    ADD PRIMARY KEY (lot_id);

ALTER TABLE coach_earnings
    DROP CONSTRAINT coach_earnings_platform_subsidy,
    DROP COLUMN platform_subsidy_yen;

ALTER TABLE review_requests
    DROP CONSTRAINT review_requests_coupon_not_tickets,
    DROP CONSTRAINT review_requests_coupon_discount,
    DROP COLUMN coupon_discount_yen;

DROP TABLE coaching_coupons;
