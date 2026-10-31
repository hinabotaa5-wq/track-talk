-- Durable full-refund operations and positive-valued coach refund debt.
ALTER TABLE review_payments DROP CONSTRAINT review_payments_status_check;
ALTER TABLE review_payments ADD CONSTRAINT review_payments_status_check CHECK (status IN ('pending','authorized','captured','canceled','refund_pending','refunded'));
CREATE TABLE captured_payment_refunds (
 id uuid PRIMARY KEY,
 source text NOT NULL CHECK (source IN ('review','consultation')),
 payment_id uuid NOT NULL,
 payment_intent_id text NOT NULL,
 amount_yen integer NOT NULL CHECK (amount_yen > 0),
 reason text NOT NULL CHECK (char_length(reason) BETWEEN 1 AND 1000),
 admin_id uuid REFERENCES profiles(id) ON DELETE SET NULL,
 origin text NOT NULL CHECK (origin IN ('admin','stripe')),
 status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','succeeded','failed')),
 stripe_refund_id text UNIQUE,
 failure text,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX captured_payment_refunds_active_idx ON captured_payment_refunds(source,payment_id) WHERE status <> 'failed';
-- Exclusions retain the original earning rather than deleting accounting history.
CREATE TABLE captured_refund_earnings (
 earning_id uuid PRIMARY KEY REFERENCES coach_earnings(id),
 refund_id uuid NOT NULL REFERENCES captured_payment_refunds(id),
 excluded boolean NOT NULL
);
CREATE TABLE coach_refund_adjustments (
 id uuid PRIMARY KEY,
 refund_id uuid NOT NULL REFERENCES captured_payment_refunds(id),
 earning_id uuid NOT NULL UNIQUE REFERENCES coach_earnings(id),
 coach_id uuid REFERENCES profiles(id) ON DELETE SET NULL,
 kind text NOT NULL DEFAULT 'refund_offset' CHECK (kind='refund_offset'),
 amount_yen integer NOT NULL CHECK (amount_yen > 0),
 applied_yen integer NOT NULL DEFAULT 0 CHECK (applied_yen >= 0 AND applied_yen <= amount_yen),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX coach_refund_adjustments_pending_idx ON coach_refund_adjustments(coach_id) WHERE applied_yen < amount_yen;
CREATE TABLE coach_refund_applications (
 id uuid PRIMARY KEY,
 adjustment_id uuid NOT NULL REFERENCES coach_refund_adjustments(id),
 earning_id uuid NOT NULL REFERENCES coach_earnings(id),
 amount_yen integer NOT NULL CHECK (amount_yen > 0),
 -- NULL while wholly offset / carried forward: no zero-valued payout is created.
 payout_id uuid REFERENCES coach_payouts(id),
 created_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE captured_payment_refunds ENABLE ROW LEVEL SECURITY;
ALTER TABLE captured_refund_earnings ENABLE ROW LEVEL SECURITY;
ALTER TABLE coach_refund_adjustments ENABLE ROW LEVEL SECURITY;
ALTER TABLE coach_refund_applications ENABLE ROW LEVEL SECURITY;

-- Keep the payment-to-earning association after withdrawal deletes the request.
CREATE TABLE review_payment_earning_links (
 payment_id uuid PRIMARY KEY REFERENCES review_payments(id),
 earning_id uuid NOT NULL UNIQUE REFERENCES coach_earnings(id)
);
INSERT INTO review_payment_earning_links(payment_id,earning_id)
SELECT p.id,e.id FROM coach_earnings e JOIN LATERAL (
 SELECT id FROM review_payments p WHERE p.request_id=e.review_request_id AND p.status IN ('captured','refunded','refund_pending') ORDER BY p.authorized_at,p.id LIMIT 1
) p ON true WHERE e.source='review';
CREATE FUNCTION link_review_payment_earning() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.source='review' THEN
 INSERT INTO review_payment_earning_links(payment_id,earning_id)
 SELECT p.id,NEW.id FROM review_payments p WHERE p.request_id=NEW.review_request_id AND p.status IN ('captured','refunded','refund_pending') ORDER BY p.authorized_at,p.id LIMIT 1 ON CONFLICT DO NOTHING;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER link_review_payment_earning AFTER INSERT ON coach_earnings FOR EACH ROW EXECUTE FUNCTION link_review_payment_earning();
ALTER TABLE review_payment_earning_links ENABLE ROW LEVEL SECURITY;
CREATE INDEX coach_refund_applications_earning_idx ON coach_refund_applications(earning_id);
CREATE INDEX coach_refund_applications_adjustment_idx ON coach_refund_applications(adjustment_id);

-- Existing personal/support refunds remove unsent earnings. Restore debt which
-- those earnings had already offset, before allowing that existing deletion.
CREATE FUNCTION restore_deleted_earning_offsets() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF OLD.payout_id IS NULL THEN
 UPDATE coach_refund_adjustments a SET applied_yen=a.applied_yen-x.n
 FROM (SELECT adjustment_id,sum(amount_yen)::integer n FROM coach_refund_applications WHERE earning_id=OLD.id AND payout_id IS NULL GROUP BY adjustment_id) x
 WHERE a.id=x.adjustment_id;
 DELETE FROM coach_refund_applications WHERE earning_id=OLD.id AND payout_id IS NULL;
 END IF;
 RETURN OLD;
END $$;
CREATE TRIGGER restore_deleted_earning_offsets BEFORE DELETE ON coach_earnings FOR EACH ROW EXECUTE FUNCTION restore_deleted_earning_offsets();
