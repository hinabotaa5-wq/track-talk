-- 退会などで契約が消え、contract_id が NULL になった精算記録が1件でもあれば戻さない。
-- 元の形（NOT NULL・contract_id が主キー・clawback は contract_id 必須）にはその行が入らず、
-- 戻すには精算記録を消すしかないため（お金の記録を消さない）。
DO $$
BEGIN
 LOCK TABLE support_coach_advances, support_coach_settlements, support_fault_determinations, coach_refund_adjustments IN ACCESS EXCLUSIVE MODE;
 IF EXISTS (SELECT 1 FROM support_coach_advances WHERE contract_id IS NULL)
    OR EXISTS (SELECT 1 FROM support_coach_settlements WHERE contract_id IS NULL)
    OR EXISTS (SELECT 1 FROM support_fault_determinations WHERE contract_id IS NULL)
    OR EXISTS (SELECT 1 FROM coach_refund_adjustments WHERE kind = 'support_clawback' AND contract_id IS NULL) THEN
  RAISE EXCEPTION 'support settlement records detached from deleted contracts exist';
 END IF;
END $$;

ALTER TABLE coach_refund_adjustments DROP CONSTRAINT coach_refund_adjustments_contract_id_fkey;
ALTER TABLE coach_refund_adjustments ADD CONSTRAINT coach_refund_adjustments_contract_id_fkey
    FOREIGN KEY (contract_id) REFERENCES coaching_contracts (id);
ALTER TABLE coach_refund_adjustments DROP CONSTRAINT coach_refund_adjustments_source_check;
ALTER TABLE coach_refund_adjustments ADD CONSTRAINT coach_refund_adjustments_source_check
    CHECK (CASE WHEN kind = 'support_clawback' THEN contract_id IS NOT NULL AND refund_id IS NULL ELSE refund_id IS NOT NULL END);

ALTER TABLE support_fault_determinations DROP CONSTRAINT support_fault_determinations_contract_id_fkey;
ALTER TABLE support_fault_determinations ADD CONSTRAINT support_fault_determinations_contract_id_fkey
    FOREIGN KEY (contract_id) REFERENCES coaching_contracts (id);
ALTER TABLE support_fault_determinations ALTER COLUMN contract_id SET NOT NULL;
ALTER TABLE support_fault_determinations DROP CONSTRAINT support_fault_determinations_contract_id_key;
ALTER TABLE support_fault_determinations DROP CONSTRAINT support_fault_determinations_pkey;
ALTER TABLE support_fault_determinations ADD CONSTRAINT support_fault_determinations_pkey PRIMARY KEY (contract_id);
ALTER TABLE support_fault_determinations DROP COLUMN id;

ALTER TABLE support_coach_settlements DROP CONSTRAINT support_coach_settlements_contract_id_fkey;
ALTER TABLE support_coach_settlements ADD CONSTRAINT support_coach_settlements_contract_id_fkey
    FOREIGN KEY (contract_id) REFERENCES coaching_contracts (id);
ALTER TABLE support_coach_settlements ALTER COLUMN contract_id SET NOT NULL;
ALTER TABLE support_coach_settlements DROP CONSTRAINT support_coach_settlements_contract_id_key;
ALTER TABLE support_coach_settlements DROP CONSTRAINT support_coach_settlements_pkey;
ALTER TABLE support_coach_settlements ADD CONSTRAINT support_coach_settlements_pkey PRIMARY KEY (contract_id);
ALTER TABLE support_coach_settlements DROP COLUMN id;

ALTER TABLE support_coach_advances DROP CONSTRAINT support_coach_advances_contract_id_fkey;
ALTER TABLE support_coach_advances ADD CONSTRAINT support_coach_advances_contract_id_fkey
    FOREIGN KEY (contract_id) REFERENCES coaching_contracts (id);
ALTER TABLE support_coach_advances ALTER COLUMN contract_id SET NOT NULL;
