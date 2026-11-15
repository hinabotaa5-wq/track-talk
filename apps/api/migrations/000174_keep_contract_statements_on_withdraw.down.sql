-- 退会などで契約が消え、contract_id が NULL になった取引条件の明示が1件でもあれば戻さない。
-- 元の形（NOT NULL・contract_id が主キー・CASCADE）にはその行が入らず、
-- 戻すには取引条件の明示を消すしかないため（取引の証跡を消さない）。
DO $$
BEGIN
 LOCK TABLE support_contract_statements IN ACCESS EXCLUSIVE MODE;
 IF EXISTS (SELECT 1 FROM support_contract_statements WHERE contract_id IS NULL) THEN
  RAISE EXCEPTION 'contract statement records detached from deleted contracts exist';
 END IF;
END $$;

ALTER TABLE support_contract_statements DROP CONSTRAINT support_contract_statements_contract_id_fkey;
ALTER TABLE support_contract_statements ADD CONSTRAINT support_contract_statements_contract_id_fkey
    FOREIGN KEY (contract_id) REFERENCES coaching_contracts (id) ON DELETE CASCADE;
ALTER TABLE support_contract_statements ALTER COLUMN contract_id SET NOT NULL;
ALTER TABLE support_contract_statements DROP CONSTRAINT support_contract_statements_contract_id_key;
ALTER TABLE support_contract_statements DROP CONSTRAINT support_contract_statements_pkey;
ALTER TABLE support_contract_statements ADD CONSTRAINT support_contract_statements_pkey PRIMARY KEY (contract_id);
ALTER TABLE support_contract_statements DROP COLUMN id;
