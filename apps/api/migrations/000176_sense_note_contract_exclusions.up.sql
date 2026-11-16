-- 継続プラン（period）で、選手が担当コーチに見せない感覚メモ（チェックを外したもの）の記録。
-- 行が無いことが「共有中」を表すため、契約中に新しく書いたメモは追加の処理なしで共有中になる。
CREATE TABLE sense_note_contract_exclusions (
    contract_id uuid NOT NULL REFERENCES coaching_contracts (id) ON DELETE CASCADE,
    sense_note_id uuid NOT NULL REFERENCES sense_notes (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (contract_id, sense_note_id)
);
CREATE INDEX sense_note_contract_exclusions_note_idx ON sense_note_contract_exclusions (sense_note_id);
ALTER TABLE sense_note_contract_exclusions ENABLE ROW LEVEL SECURITY;
