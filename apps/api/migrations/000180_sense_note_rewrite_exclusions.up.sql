-- 継続プラン（period）で、選手がチェックを外していた感覚メモを同じ日に消して書き直したとき、
-- 新しいメモにも同じ契約の除外を引き継ぐための記録。メモを消すと sense_note_contract_exclusions の行も
-- 一緒に消えるので、消す時点で除外があった契約と日付（日本時間）だけをここに残す。本文は残さない。
-- 同じ選手が次に感覚メモを作る（または消す）ときに読み捨てる。日付が違う記録は引き継ぎに使わない。
CREATE TABLE sense_note_rewrite_exclusions (
    author_id uuid NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    contract_id uuid NOT NULL REFERENCES coaching_contracts (id) ON DELETE CASCADE,
    day date NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (author_id, day, contract_id)
);
CREATE INDEX sense_note_rewrite_exclusions_contract_idx ON sense_note_rewrite_exclusions (contract_id);
ALTER TABLE sense_note_rewrite_exclusions ENABLE ROW LEVEL SECURITY;
