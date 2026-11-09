-- 記録が1件でもあれば戻さない（お金の記録を消さないため）。
DO $$
BEGIN
 LOCK TABLE support_coach_advances, support_coach_settlements, support_fault_determinations, support_contract_statements, coach_refund_adjustments IN ACCESS EXCLUSIVE MODE;
 IF EXISTS (SELECT 1 FROM support_coach_advances) OR EXISTS (SELECT 1 FROM support_coach_settlements)
    OR EXISTS (SELECT 1 FROM support_fault_determinations) OR EXISTS (SELECT 1 FROM support_contract_statements)
    OR EXISTS (SELECT 1 FROM coach_refund_adjustments WHERE kind = 'support_clawback')
    OR EXISTS (SELECT 1 FROM coach_earnings WHERE source = 'support_advance')
    OR EXISTS (SELECT 1 FROM coaching_contracts WHERE support_end_cause IS NOT NULL OR support_coach_hold)
    OR EXISTS (SELECT 1 FROM coaching_payments WHERE refund_reason IN ('support_player', 'support_coach_fault', 'support_unavoidable', 'support_competition_cancelled')) THEN
  RAISE EXCEPTION 'support settlement records exist';
 END IF;
END $$;
DROP TABLE support_contract_statements, support_fault_determinations, support_coach_settlements, support_coach_advances;
DROP INDEX coach_refund_adjustments_support_idx;
ALTER TABLE coach_refund_adjustments DROP CONSTRAINT coach_refund_adjustments_source_check;
ALTER TABLE coach_refund_adjustments DROP CONSTRAINT coach_refund_adjustments_earning_required;
ALTER TABLE coach_refund_adjustments ADD CONSTRAINT coach_refund_adjustments_earning_required CHECK (kind = 'compensation_recovery' OR earning_id IS NOT NULL);
ALTER TABLE coach_refund_adjustments DROP CONSTRAINT coach_refund_adjustments_kind_check;
ALTER TABLE coach_refund_adjustments ADD CONSTRAINT coach_refund_adjustments_kind_check CHECK (kind IN ('refund_offset', 'compensation_recovery'));
ALTER TABLE coach_refund_adjustments DROP COLUMN contract_id;
ALTER TABLE coach_refund_adjustments ALTER COLUMN refund_id SET NOT NULL;
ALTER TABLE coach_earnings DROP CONSTRAINT coach_earnings_source_check;
ALTER TABLE coach_earnings ADD CONSTRAINT coach_earnings_source_check CHECK (source IN ('review', 'personal', 'consultation', 'refund_compensation'));
ALTER TABLE coaching_contracts DROP CONSTRAINT coaching_contracts_support_end_check,
    DROP COLUMN support_coach_hold, DROP COLUMN support_end_on, DROP COLUMN support_end_cause;
ALTER TABLE coaching_payments DROP CONSTRAINT coaching_payments_refund_reason_check;
ALTER TABLE coaching_payments ADD CONSTRAINT coaching_payments_refund_reason_check CHECK (refund_reason IN ('no_coach_reply', 'not_started', 'admin_full', 'admin_remaining', 'support_unprovided', 'support_admin_full', 'support_no_response', 'in_person_unprovided', 'in_person_admin_full'));
