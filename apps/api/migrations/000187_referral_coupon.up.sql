CREATE TABLE coaching_referral_coupons (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id uuid NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    granted_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    used_reservation_id uuid UNIQUE REFERENCES coaching_ticket_reservations(id) ON DELETE SET NULL,
    canceled_at timestamptz
);
CREATE INDEX coaching_referral_coupons_owner_idx ON coaching_referral_coupons(owner_id);
ALTER TABLE coaching_referral_coupons ENABLE ROW LEVEL SECURITY;

ALTER TABLE coaching_referral_rewards
    DROP CONSTRAINT coaching_referral_rewards_pkey,
    ADD COLUMN id uuid NOT NULL DEFAULT gen_random_uuid(),
    ALTER COLUMN lot_id DROP NOT NULL,
    ADD CONSTRAINT coaching_referral_rewards_pkey PRIMARY KEY (id),
    ADD CONSTRAINT coaching_referral_rewards_lot_id_key UNIQUE (lot_id),
    ADD COLUMN coupon_id uuid UNIQUE REFERENCES coaching_referral_coupons(id) ON DELETE CASCADE,
    ADD CONSTRAINT coaching_referral_reward_source CHECK (num_nonnulls(lot_id, coupon_id) = 1);

ALTER TABLE coach_earnings ADD COLUMN reservation_id uuid
    REFERENCES coaching_ticket_reservations(id) ON DELETE SET NULL;
UPDATE coach_earnings e SET reservation_id = t.id
FROM coaching_ticket_reservations t
WHERE e.source = 'review' AND t.request_id = e.review_request_id
  AND NOT EXISTS (SELECT 1 FROM review_payment_earning_links l WHERE l.earning_id = e.id);

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
    INSERT INTO coaching_referral_coupons(owner_id, expires_at)
    VALUES (c.owner_id, now() + interval '6 months')
    RETURNING id INTO reward;
    INSERT INTO coaching_referral_rewards(coupon_id, referrer_id, first_count_id, second_count_id, referral_code)
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
    coupon coaching_referral_coupons;
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
        SELECT * INTO coupon FROM coaching_referral_coupons WHERE id = reward.coupon_id FOR UPDATE;
        IF coupon.used_reservation_id IS NULL THEN
            UPDATE coaching_referral_coupons SET canceled_at = now() WHERE id = coupon.id;
            UPDATE coaching_referral_rewards SET qualifying_refund_at = now(), canceled_at = now()
            WHERE id = reward.id;
        ELSIF EXISTS (
            SELECT 1 FROM coaching_ticket_reservations WHERE id = coupon.used_reservation_id AND status = 'reserved'
        ) THEN
            UPDATE coaching_referral_rewards SET qualifying_refund_at = now() WHERE id = reward.id;
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
