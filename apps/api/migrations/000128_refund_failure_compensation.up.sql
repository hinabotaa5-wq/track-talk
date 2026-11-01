ALTER TABLE captured_payment_refunds ADD COLUMN succeeded_at timestamptz, ADD COLUMN reconciled_at timestamptz;
UPDATE captured_payment_refunds SET succeeded_at=updated_at WHERE status='succeeded';
CREATE INDEX captured_refund_reconciliation_idx ON captured_payment_refunds(status,reconciled_at);
ALTER TABLE coach_refund_adjustments DROP CONSTRAINT coach_refund_adjustments_earning_id_key;
ALTER TABLE coach_refund_adjustments ALTER COLUMN earning_id DROP NOT NULL;
ALTER TABLE coach_refund_adjustments DROP CONSTRAINT coach_refund_adjustments_kind_check;
ALTER TABLE coach_refund_adjustments ADD CONSTRAINT coach_refund_adjustments_kind_check CHECK (kind IN ('refund_offset','compensation_recovery'));
ALTER TABLE coach_refund_adjustments ADD CONSTRAINT coach_refund_adjustments_earning_required CHECK(kind='compensation_recovery' OR earning_id IS NOT NULL);
ALTER TABLE coach_refund_adjustments ADD CONSTRAINT coach_refund_adjustments_operation_earning_key UNIQUE(refund_id,earning_id,kind);
ALTER TABLE coach_earnings DROP CONSTRAINT coach_earnings_source_check;
ALTER TABLE coach_earnings ADD CONSTRAINT coach_earnings_source_check CHECK (source IN ('review','personal','consultation','refund_compensation'));
CREATE TABLE coach_refund_cancellations (
 id uuid PRIMARY KEY,
 adjustment_id uuid NOT NULL UNIQUE REFERENCES coach_refund_adjustments(id),
 refund_id uuid NOT NULL REFERENCES captured_payment_refunds(id),
 amount_yen integer NOT NULL CHECK(amount_yen>0),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE coach_refund_compensations (
 id uuid PRIMARY KEY,
 adjustment_id uuid NOT NULL UNIQUE REFERENCES coach_refund_adjustments(id),
 refund_id uuid NOT NULL REFERENCES captured_payment_refunds(id),
 earning_id uuid NOT NULL UNIQUE REFERENCES coach_earnings(id),
 amount_yen integer NOT NULL CHECK(amount_yen>0),
 created_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE coach_refund_applications ADD COLUMN earning_reference_id uuid;
UPDATE coach_refund_applications SET earning_reference_id=earning_id;
ALTER TABLE coach_refund_applications ALTER COLUMN earning_reference_id SET NOT NULL;
ALTER TABLE coach_refund_applications ALTER COLUMN earning_id DROP NOT NULL;
ALTER TABLE coach_refund_applications DROP CONSTRAINT coach_refund_applications_earning_id_fkey;
ALTER TABLE coach_refund_applications ADD CONSTRAINT coach_refund_applications_earning_id_fkey FOREIGN KEY(earning_id) REFERENCES coach_earnings(id) ON DELETE SET NULL;
CREATE FUNCTION snapshot_refund_application() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN NEW.earning_reference_id=NEW.earning_id; RETURN NEW; END $$;
CREATE TRIGGER snapshot_refund_application BEFORE INSERT ON coach_refund_applications FOR EACH ROW EXECUTE FUNCTION snapshot_refund_application();
CREATE TABLE coach_refund_application_reversals (
 id uuid PRIMARY KEY,
 application_id uuid NOT NULL UNIQUE REFERENCES coach_refund_applications(id),
 recovery_adjustment_id uuid UNIQUE REFERENCES coach_refund_adjustments(id),
 created_at timestamptz NOT NULL DEFAULT now()
);
-- Retain application history. If a canceled debt's consideration is later
-- refunded, recover its already-issued compensation instead of reviving it.
CREATE FUNCTION reverse_coach_refund_applications(target uuid) RETURNS void LANGUAGE plpgsql AS $$
DECLARE x record; recovery uuid;
BEGIN
 FOR x IN SELECT p.id,p.adjustment_id,p.amount_yen,a.refund_id,a.coach_id FROM coach_refund_applications p JOIN coach_refund_adjustments a ON a.id=p.adjustment_id
 WHERE p.earning_id=target AND p.payout_id IS NULL AND NOT EXISTS(SELECT 1 FROM coach_refund_application_reversals v WHERE v.application_id=p.id) FOR UPDATE OF p,a
 LOOP
 recovery=NULL;
 IF EXISTS(SELECT 1 FROM coach_refund_cancellations c WHERE c.adjustment_id=x.adjustment_id) OR EXISTS(SELECT 1 FROM coach_refund_compensations c WHERE c.adjustment_id=x.adjustment_id) THEN
 recovery=gen_random_uuid();
 INSERT INTO coach_refund_adjustments(id,refund_id,coach_id,kind,amount_yen) VALUES(recovery,x.refund_id,x.coach_id,'compensation_recovery',x.amount_yen);
 ELSE
 UPDATE coach_refund_adjustments SET applied_yen=applied_yen-x.amount_yen WHERE id=x.adjustment_id;
 END IF;
 INSERT INTO coach_refund_application_reversals(id,application_id,recovery_adjustment_id) VALUES(gen_random_uuid(),x.id,recovery);
 END LOOP;
END $$;
CREATE OR REPLACE FUNCTION restore_deleted_earning_offsets() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF OLD.payout_id IS NULL THEN PERFORM reverse_coach_refund_applications(OLD.id); END IF;
 RETURN OLD;
END $$;
ALTER TABLE coach_refund_cancellations ENABLE ROW LEVEL SECURITY;
ALTER TABLE coach_refund_compensations ENABLE ROW LEVEL SECURITY;
ALTER TABLE coach_refund_application_reversals ENABLE ROW LEVEL SECURITY;
