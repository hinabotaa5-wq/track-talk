-- #726: 精算まで終えた1ヶ月プランがある利用者の退会（DELETE /v1/me）が、契約の CASCADE 削除の途中で
-- 外部キー違反（23503）になり 422 で失敗していた。運営者決定（2026-11-15）で、精算記録（仮払い・確定額・
-- 帰責判定・返還の台帳）は「取引に関する記録」として退会後も残す。契約への参照は ON DELETE SET NULL にして
-- 契約側の削除と切り離す（coaching_payments が支払記録を SET NULL で残しているのと揃える）。CASCADE にはしない。

-- 月次の仮払い。主キーは earning_id なので、contract_id の NOT NULL を外すだけでよい。
ALTER TABLE support_coach_advances ALTER COLUMN contract_id DROP NOT NULL;
ALTER TABLE support_coach_advances DROP CONSTRAINT support_coach_advances_contract_id_fkey;
ALTER TABLE support_coach_advances ADD CONSTRAINT support_coach_advances_contract_id_fkey
    FOREIGN KEY (contract_id) REFERENCES coaching_contracts (id) ON DELETE SET NULL;

-- 契約終了時の確定と、帰責の判定は contract_id が主キーで、NULL にできない。
-- 行を識別する id を主キーにし、契約ごとに1行は UNIQUE (contract_id) で保つ（NULL どうしは重ならない扱い）。
ALTER TABLE support_coach_settlements ADD COLUMN id uuid NOT NULL DEFAULT gen_random_uuid();
ALTER TABLE support_coach_settlements DROP CONSTRAINT support_coach_settlements_pkey;
ALTER TABLE support_coach_settlements ADD CONSTRAINT support_coach_settlements_pkey PRIMARY KEY (id);
ALTER TABLE support_coach_settlements ADD CONSTRAINT support_coach_settlements_contract_id_key UNIQUE (contract_id);
ALTER TABLE support_coach_settlements ALTER COLUMN contract_id DROP NOT NULL;
ALTER TABLE support_coach_settlements DROP CONSTRAINT support_coach_settlements_contract_id_fkey;
ALTER TABLE support_coach_settlements ADD CONSTRAINT support_coach_settlements_contract_id_fkey
    FOREIGN KEY (contract_id) REFERENCES coaching_contracts (id) ON DELETE SET NULL;

ALTER TABLE support_fault_determinations ADD COLUMN id uuid NOT NULL DEFAULT gen_random_uuid();
ALTER TABLE support_fault_determinations DROP CONSTRAINT support_fault_determinations_pkey;
ALTER TABLE support_fault_determinations ADD CONSTRAINT support_fault_determinations_pkey PRIMARY KEY (id);
ALTER TABLE support_fault_determinations ADD CONSTRAINT support_fault_determinations_contract_id_key UNIQUE (contract_id);
ALTER TABLE support_fault_determinations ALTER COLUMN contract_id DROP NOT NULL;
ALTER TABLE support_fault_determinations DROP CONSTRAINT support_fault_determinations_contract_id_fkey;
ALTER TABLE support_fault_determinations ADD CONSTRAINT support_fault_determinations_contract_id_fkey
    FOREIGN KEY (contract_id) REFERENCES coaching_contracts (id) ON DELETE SET NULL;

-- 返還の台帳。contract_id はもともと NULL を許す。ただし仮払いの返還（support_clawback）の行は
-- 「contract_id が入っていること」を CHECK で求めていたため、契約が消えて NULL になると CHECK 違反になる。
-- 作るときに契約を必ず付けるのはアプリ側（InsertSupportClawback）で担保し、CHECK は refund_id が空であることだけにする。
ALTER TABLE coach_refund_adjustments DROP CONSTRAINT coach_refund_adjustments_source_check;
ALTER TABLE coach_refund_adjustments ADD CONSTRAINT coach_refund_adjustments_source_check
    CHECK (CASE WHEN kind = 'support_clawback' THEN refund_id IS NULL ELSE refund_id IS NOT NULL END);
ALTER TABLE coach_refund_adjustments DROP CONSTRAINT coach_refund_adjustments_contract_id_fkey;
ALTER TABLE coach_refund_adjustments ADD CONSTRAINT coach_refund_adjustments_contract_id_fkey
    FOREIGN KEY (contract_id) REFERENCES coaching_contracts (id) ON DELETE SET NULL;
