-- A downgrade with issued rewards would discard ticket history.
DO $$
BEGIN
    IF EXISTS (SELECT FROM coaching_ticket_lots WHERE kind = 'referral_reward') THEN
        RAISE EXCEPTION 'Cannot remove referrals after reward lots have been issued';
    END IF;
END
$$;
DROP TRIGGER coaching_review_referral_refund ON review_payments;
DROP TRIGGER coaching_ticket_referral_refund ON coaching_ticket_purchases;
DROP FUNCTION coaching_cancel_referral();
DROP FUNCTION coaching_count_referral(uuid, text, uuid, uuid);
DROP FUNCTION coaching_issue_referral(uuid, text);
DROP TABLE coaching_referral_rewards, coaching_referral_counts;
ALTER TABLE review_payments DROP COLUMN referral_code;
ALTER TABLE coaching_ticket_purchases DROP COLUMN referral_code;
DROP TABLE coaching_referral_codes;
ALTER TABLE coaching_ticket_ledger DROP CONSTRAINT coaching_ticket_ledger_kind_check;
ALTER TABLE coaching_ticket_ledger ADD CONSTRAINT coaching_ticket_ledger_kind_check
    CHECK (kind IN ('purchase', 'reserve', 'consume', 'release', 'refund', 'refund_failed', 'expire'));
DROP TRIGGER coaching_default_ticket_purchase ON coaching_ticket_lots;
DROP FUNCTION coaching_default_ticket_purchase();
ALTER TABLE coaching_ticket_lots ADD CONSTRAINT coaching_ticket_lots_id_fkey
    FOREIGN KEY (id) REFERENCES coaching_ticket_purchases(id);
ALTER TABLE coaching_ticket_lots DROP CONSTRAINT coaching_ticket_lot_source,
    DROP COLUMN kind, DROP COLUMN purchase_id;
