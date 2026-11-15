-- #748: 取引条件の明示（フリーランス法第3条）は「取引に関する記録」として退会後も残す。
-- 契約への参照を ON DELETE SET NULL にして契約の削除と切り離す（migration 000172 の
-- support_coach_settlements／support_fault_determinations と同型）。CASCADE にはしない。
-- 契約ごとに1行は UNIQUE (contract_id) で保つ（NULL どうしは重ならない扱い）。

ALTER TABLE support_contract_statements ADD COLUMN id uuid NOT NULL DEFAULT gen_random_uuid();
ALTER TABLE support_contract_statements DROP CONSTRAINT support_contract_statements_pkey;
ALTER TABLE support_contract_statements ADD CONSTRAINT support_contract_statements_pkey PRIMARY KEY (id);
ALTER TABLE support_contract_statements ADD CONSTRAINT support_contract_statements_contract_id_key UNIQUE (contract_id);
ALTER TABLE support_contract_statements ALTER COLUMN contract_id DROP NOT NULL;
ALTER TABLE support_contract_statements DROP CONSTRAINT support_contract_statements_contract_id_fkey;
ALTER TABLE support_contract_statements ADD CONSTRAINT support_contract_statements_contract_id_fkey
    FOREIGN KEY (contract_id) REFERENCES coaching_contracts (id) ON DELETE SET NULL;
