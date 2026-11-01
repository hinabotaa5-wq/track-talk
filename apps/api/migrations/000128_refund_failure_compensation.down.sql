-- External refunds and accounting history cannot be undone by schema rollback.
DO $$ BEGIN
 PERFORM pg_advisory_xact_lock(hashtext('coach_payouts'));
 LOCK TABLE captured_payment_refunds,captured_refund_earnings,coach_refund_adjustments,coach_refund_applications IN ACCESS EXCLUSIVE MODE;
 IF EXISTS(SELECT 1 FROM captured_payment_refunds) OR EXISTS(SELECT 1 FROM captured_refund_earnings) OR EXISTS(SELECT 1 FROM coach_refund_adjustments) OR EXISTS(SELECT 1 FROM coach_refund_applications) THEN
 RAISE EXCEPTION 'refund accounting exists: rollback refused; reconcile and roll forward';
 END IF;
END $$;

DROP TABLE coach_refund_application_reversals;
DROP FUNCTION reverse_coach_refund_applications(uuid);
DROP TRIGGER snapshot_refund_application ON coach_refund_applications;
DROP FUNCTION snapshot_refund_application();
DROP TABLE coach_refund_compensations,coach_refund_cancellations;
ALTER TABLE coach_refund_applications DROP CONSTRAINT coach_refund_applications_earning_id_fkey;
ALTER TABLE coach_refund_applications ALTER COLUMN earning_id SET NOT NULL;
ALTER TABLE coach_refund_applications ADD FOREIGN KEY(earning_id) REFERENCES coach_earnings(id);
ALTER TABLE coach_refund_applications DROP COLUMN earning_reference_id;
ALTER TABLE coach_refund_adjustments DROP CONSTRAINT coach_refund_adjustments_earning_required;
ALTER TABLE coach_refund_adjustments DROP CONSTRAINT coach_refund_adjustments_operation_earning_key;
ALTER TABLE coach_refund_adjustments DROP CONSTRAINT coach_refund_adjustments_kind_check;
ALTER TABLE coach_refund_adjustments ADD CHECK(kind='refund_offset');
ALTER TABLE coach_refund_adjustments ALTER COLUMN earning_id SET NOT NULL;
ALTER TABLE coach_refund_adjustments ADD UNIQUE(earning_id);
ALTER TABLE coach_earnings DROP CONSTRAINT coach_earnings_source_check;
ALTER TABLE coach_earnings ADD CONSTRAINT coach_earnings_source_check CHECK(source IN ('review','personal','consultation'));
ALTER TABLE captured_payment_refunds DROP COLUMN succeeded_at,DROP COLUMN reconciled_at;
CREATE OR REPLACE FUNCTION restore_deleted_earning_offsets() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF OLD.payout_id IS NULL THEN
 UPDATE coach_refund_adjustments a SET applied_yen=a.applied_yen-x.n FROM (SELECT adjustment_id,sum(amount_yen)::integer n FROM coach_refund_applications WHERE earning_id=OLD.id AND payout_id IS NULL GROUP BY adjustment_id) x WHERE a.id=x.adjustment_id;
 DELETE FROM coach_refund_applications WHERE earning_id=OLD.id AND payout_id IS NULL;
 END IF; RETURN OLD; END $$;
