ALTER TABLE coaching_ticket_lots ADD COLUMN purchase_id uuid UNIQUE REFERENCES coaching_ticket_purchases(id);
ALTER TABLE coaching_ticket_lots ADD COLUMN kind text NOT NULL DEFAULT 'purchase';
UPDATE coaching_ticket_lots SET purchase_id = id;
ALTER TABLE coaching_ticket_lots DROP CONSTRAINT coaching_ticket_lots_id_fkey,
    ADD CONSTRAINT coaching_ticket_lot_source CHECK (
        (kind = 'purchase' AND purchase_id IS NOT NULL) OR
        (kind = 'referral_reward' AND purchase_id IS NULL AND quantity = 1)
    );

-- Keep the pre-migration API compatible during the migration-first rollout.
CREATE FUNCTION coaching_default_ticket_purchase() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.kind = 'purchase' AND NEW.purchase_id IS NULL THEN
        NEW.purchase_id := NEW.id;
    END IF;
    RETURN NEW;
END
$$;
CREATE TRIGGER coaching_default_ticket_purchase BEFORE INSERT ON coaching_ticket_lots
    FOR EACH ROW EXECUTE FUNCTION coaching_default_ticket_purchase();

CREATE TABLE coaching_referral_codes (
    owner_id uuid PRIMARY KEY REFERENCES profiles(id) ON DELETE CASCADE,
    code text NOT NULL UNIQUE CHECK (code ~ '^[23456789ABCDEFGHJKLMNPQRSTUVWXYZ]{6}$'),
    issued_at timestamptz NOT NULL DEFAULT now(),
    issued_by text NOT NULL CHECK (issued_by IN ('purchase', 'backfill'))
);
ALTER TABLE coaching_ticket_purchases ADD COLUMN referral_code text REFERENCES coaching_referral_codes(code) ON DELETE SET NULL;
ALTER TABLE review_payments ADD COLUMN referral_code text REFERENCES coaching_referral_codes(code) ON DELETE SET NULL;

CREATE TABLE coaching_referral_counts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    referrer_id uuid REFERENCES profiles(id) ON DELETE SET NULL,
    buyer_id uuid REFERENCES profiles(id) ON DELETE SET NULL,
    ticket_purchase_id uuid UNIQUE REFERENCES coaching_ticket_purchases(id),
    review_payment_id uuid UNIQUE REFERENCES review_payments(id) ON DELETE CASCADE,
    counted_at timestamptz NOT NULL DEFAULT now(),
    canceled_at timestamptz,
    UNIQUE (referrer_id, buyer_id),
    CHECK (num_nonnulls(ticket_purchase_id, review_payment_id) = 1),
    CHECK (referrer_id <> buyer_id)
);
CREATE TABLE coaching_referral_rewards (
    lot_id uuid PRIMARY KEY REFERENCES coaching_ticket_lots(id),
    referrer_id uuid REFERENCES profiles(id) ON DELETE SET NULL,
    first_count_id uuid REFERENCES coaching_referral_counts(id) ON DELETE SET NULL,
    second_count_id uuid REFERENCES coaching_referral_counts(id) ON DELETE SET NULL,
    granted_at timestamptz NOT NULL DEFAULT now(),
    canceled_at timestamptz,
    qualifying_refund_at timestamptz
);
CREATE UNIQUE INDEX coaching_referral_active_reward
    ON coaching_referral_rewards(referrer_id) WHERE canceled_at IS NULL;

ALTER TABLE coaching_referral_codes ENABLE ROW LEVEL SECURITY;
ALTER TABLE coaching_referral_counts ENABLE ROW LEVEL SECURITY;
ALTER TABLE coaching_referral_rewards ENABLE ROW LEVEL SECURITY;

-- A UUID supplies six independent random bytes. The alphabet has 32 symbols,
-- so reducing a byte modulo 32 does not bias the generated code.
CREATE FUNCTION coaching_issue_referral(owner uuid, source text) RETURNS void LANGUAGE plpgsql AS $$
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

DO $$
DECLARE buyer uuid;
BEGIN
    FOR buyer IN
        SELECT owner_id FROM coaching_ticket_purchases
        WHERE product_id IN ('single', 'six') AND paid_at IS NOT NULL AND owner_id IS NOT NULL
        UNION
        SELECT payer_id FROM review_payments
        WHERE amount_yen = 500 AND captured_at IS NOT NULL AND payer_id IS NOT NULL
    LOOP
        PERFORM coaching_issue_referral(buyer, 'backfill');
    END LOOP;
END
$$;

-- Lock the referrer's code row for all count/reward writes. Issuing the
-- buyer's own code is separate from this lock to avoid reciprocal referrals
-- taking two code-row locks in opposite order.
CREATE FUNCTION coaching_count_referral(buyer uuid, entered_code text, ticket_id uuid, payment_id uuid)
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

ALTER TABLE coaching_ticket_ledger DROP CONSTRAINT coaching_ticket_ledger_kind_check;
ALTER TABLE coaching_ticket_ledger ADD CONSTRAINT coaching_ticket_ledger_kind_check
    CHECK (kind IN ('purchase', 'reserve', 'consume', 'release', 'refund', 'refund_failed', 'expire', 'referral_grant'));

CREATE FUNCTION coaching_cancel_referral() RETURNS trigger LANGUAGE plpgsql AS $$
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
    -- Honor an existing reservation. On release, the API checks this
    -- recorded refund and discards the ticket instead of restoring it.
    IF lot.available > 0 THEN
        UPDATE coaching_ticket_lots SET available = 0, expired = expired + lot.available
        WHERE id = lot.id;
        INSERT INTO coaching_ticket_ledger(id, lot_id, operation_key, kind, available_delta, expired_delta)
        VALUES (gen_random_uuid(), lot.id, 'referral-cancel-' || lot.id, 'expire', -lot.available, lot.available);
    END IF;
    RETURN NEW;
END
$$;

CREATE TRIGGER coaching_ticket_referral_refund AFTER UPDATE OF refunded_yen ON coaching_ticket_purchases
    FOR EACH ROW EXECUTE FUNCTION coaching_cancel_referral();
CREATE TRIGGER coaching_review_referral_refund AFTER UPDATE OF status ON review_payments
    FOR EACH ROW EXECUTE FUNCTION coaching_cancel_referral();
