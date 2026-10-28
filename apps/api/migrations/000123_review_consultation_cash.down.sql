-- 都度払いのフォーム添削の追加相談があると戻せない(記録を消さないため)。
ALTER TABLE coaching_consultations DROP CONSTRAINT coaching_consultations_funding_parent_check;
ALTER TABLE coaching_consultations ADD CONSTRAINT coaching_consultations_check1
    CHECK ((review_request_id IS NOT NULL AND funding='ticket') OR (contract_id IS NOT NULL AND funding='cash'));
