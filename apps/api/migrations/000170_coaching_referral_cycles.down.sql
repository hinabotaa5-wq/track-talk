-- 2周目以降のIDが発行された後は1人1行に戻せないため、戻さずに止める。
DO $$
BEGIN
    IF EXISTS (SELECT FROM coaching_referral_codes GROUP BY owner_id HAVING count(*) > 1) THEN
        RAISE EXCEPTION 'Cannot remove referral cycles after a second referral code has been issued';
    END IF;
    IF EXISTS (SELECT FROM coaching_referral_counts WHERE referrer_id IS NOT NULL AND buyer_id IS NOT NULL
               GROUP BY referrer_id, buyer_id HAVING count(*) > 1) THEN
        RAISE EXCEPTION 'Cannot remove referral cycles after a refunded buyer has been counted again';
    END IF;
END
$$;

DROP TRIGGER coaching_referral_reward_canceled ON coaching_referral_rewards;
DROP FUNCTION coaching_reopen_referral_cycle();
DROP FUNCTION coaching_cancel_referral_for_payment_intent(text);
DROP FUNCTION coaching_record_referral(uuid, text, uuid, uuid);

-- 000167 の関数本体に戻す。
CREATE OR REPLACE FUNCTION coaching_cancel_referral() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    contribution coaching_referral_counts;
    reward coaching_referral_rewards;
    lot coaching_ticket_lots;
BEGIN
    IF TG_TABLE_NAME = 'review_payments' THEN
        IF NEW.status <> 'refunded' OR OLD.status = 'refunded' THEN
            RETURN NEW;
        END IF;
        SELECT * INTO contribution FROM coaching_referral_counts WHERE review_payment_id = NEW.id;
    ELSE
        IF NEW.refunded_yen <= OLD.refunded_yen THEN
            RETURN NEW;
        END IF;
        SELECT * INTO contribution FROM coaching_referral_counts WHERE ticket_purchase_id = NEW.id;
    END IF;
    IF contribution.id IS NULL THEN
        RETURN NEW;
    END IF;
    PERFORM 1 FROM coaching_referral_codes WHERE owner_id = contribution.referrer_id FOR NO KEY UPDATE;
    UPDATE coaching_referral_counts SET canceled_at = now() WHERE id = contribution.id AND canceled_at IS NULL;
    IF NOT FOUND THEN
        RETURN NEW;
    END IF;

    SELECT * INTO reward FROM coaching_referral_rewards
        WHERE referrer_id = contribution.referrer_id AND canceled_at IS NULL
          AND contribution.id IN (first_count_id, second_count_id);
    IF NOT FOUND THEN
        RETURN NEW;
    END IF;
    SELECT * INTO lot FROM coaching_ticket_lots WHERE id = reward.lot_id FOR UPDATE;
    UPDATE coaching_referral_rewards SET qualifying_refund_at = now(),
        canceled_at = CASE WHEN lot.consumed = 0 AND lot.reserved = 0 THEN now() END
    WHERE lot_id = lot.id;
    IF lot.available > 0 THEN
        UPDATE coaching_ticket_lots SET available = 0, expired = expired + lot.available
        WHERE id = lot.id;
        INSERT INTO coaching_ticket_ledger(id, lot_id, operation_key, kind, available_delta, expired_delta)
        VALUES (gen_random_uuid(), lot.id, 'referral-cancel-' || lot.id, 'expire', -lot.available, lot.available);
    END IF;
    RETURN NEW;
END
$$;
DROP FUNCTION coaching_cancel_referral_count(uuid);

CREATE OR REPLACE FUNCTION coaching_count_referral(buyer uuid, entered_code text, ticket_id uuid, payment_id uuid)
RETURNS uuid LANGUAGE plpgsql AS $$
DECLARE
    referrer uuid;
    qualified uuid[];
    reward uuid;
BEGIN
    SELECT owner_id INTO referrer FROM coaching_referral_codes WHERE code = entered_code FOR NO KEY UPDATE;
    IF referrer IS NULL OR referrer = buyer OR EXISTS (
        SELECT FROM coaching_referral_rewards WHERE referrer_id = referrer AND canceled_at IS NULL
    ) THEN
        RETURN NULL;
    END IF;
    INSERT INTO coaching_referral_counts(referrer_id, buyer_id, ticket_purchase_id, review_payment_id)
    VALUES (referrer, buyer, ticket_id, payment_id) ON CONFLICT DO NOTHING;
    IF NOT FOUND THEN
        RETURN NULL;
    END IF;
    SELECT array_agg(id ORDER BY counted_at, id) INTO qualified
        FROM coaching_referral_counts WHERE referrer_id = referrer AND canceled_at IS NULL;
    IF cardinality(qualified) < 2 THEN
        RETURN NULL;
    END IF;
    reward := gen_random_uuid();
    INSERT INTO coaching_ticket_lots(id, owner_id, quantity, available, expires_at, kind)
    VALUES (reward, referrer, 1, 1, now() + interval '6 months', 'referral_reward');
    INSERT INTO coaching_ticket_ledger(id, lot_id, operation_key, kind, available_delta)
    VALUES (gen_random_uuid(), reward, 'referral-grant-' || reward, 'referral_grant', 1);
    INSERT INTO coaching_referral_rewards(lot_id, referrer_id, first_count_id, second_count_id)
    VALUES (reward, referrer, qualified[1], qualified[2]);
    RETURN referrer;
END
$$;
DROP FUNCTION coaching_grant_referral(text);

CREATE OR REPLACE FUNCTION coaching_issue_referral(owner uuid, source text) RETURNS void LANGUAGE plpgsql AS $$
DECLARE
    alphabet constant text := '23456789ABCDEFGHJKLMNPQRSTUVWXYZ';
    entropy bytea;
    candidate text;
BEGIN
    WHILE NOT EXISTS (SELECT FROM coaching_referral_codes WHERE owner_id = owner) LOOP
        entropy := uuid_send(gen_random_uuid());
        SELECT string_agg(substr(alphabet, 1 + get_byte(entropy, n) % length(alphabet), 1), '' ORDER BY n)
            INTO candidate FROM generate_series(0, 5) n;
        INSERT INTO coaching_referral_codes(owner_id, code, issued_by)
        VALUES (owner, candidate, source) ON CONFLICT DO NOTHING;
    END LOOP;
END
$$;
DROP FUNCTION coaching_issue_referral_code(uuid, text, uuid, uuid);

DROP INDEX coaching_referral_active_reward;
CREATE UNIQUE INDEX coaching_referral_active_reward
    ON coaching_referral_rewards(referrer_id) WHERE canceled_at IS NULL;
ALTER TABLE coaching_referral_rewards DROP COLUMN referral_code;

DROP INDEX coaching_referral_counts_valid_pair;
ALTER TABLE coaching_referral_counts ADD CONSTRAINT coaching_referral_counts_referrer_id_buyer_id_key
    UNIQUE (referrer_id, buyer_id);
DROP INDEX coaching_referral_counts_code_idx;
ALTER TABLE coaching_referral_counts DROP COLUMN referral_code;

DROP INDEX coaching_referral_codes_owner_idx;
DROP INDEX coaching_referral_active_code;
ALTER TABLE coaching_referral_codes DROP CONSTRAINT coaching_referral_code_origin,
    DROP COLUMN origin_review_payment_id, DROP COLUMN origin_ticket_purchase_id, DROP COLUMN voided_at,
    DROP COLUMN closed_at;
ALTER TABLE coaching_referral_codes ADD PRIMARY KEY (owner_id);
