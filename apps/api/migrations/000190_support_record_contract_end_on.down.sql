-- 書き写しの仕組みと控え列だけを外す。期限で消えた記録は戻らないが、残っている記録の中身は変えない。
DROP TRIGGER copy_support_record_contract_end_on ON coaching_contracts;
DROP FUNCTION copy_support_record_contract_end_on();
ALTER TABLE support_fault_determinations DROP COLUMN contract_end_on;
ALTER TABLE support_coach_advances DROP COLUMN contract_end_on;
ALTER TABLE support_coach_settlements DROP COLUMN contract_end_on;
ALTER TABLE support_contract_statements DROP COLUMN contract_end_on;
