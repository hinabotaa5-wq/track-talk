-- 台帳の購入参照の分離（本体 Phase 2 の計数点の前提の続き）。
-- 000167 でロットに purchase_id と kind が入り、ロットの購入対応はできている。
-- 台帳の行も購入ごとに集計できるよう、ledger.purchase_ref を足す（ロットの purchase_id の写し）。
-- 紹介特典のロット（kind='referral_reward'）には購入がないため、purchase_ref は
-- NULL 許容にする。購入ロット由来の行はつねに purchase_ref が入る。
-- 既存行はロットの purchase_id から写す。台帳の lot_id には外部キーがあるため、
-- 対応するロットが必ず存在する。

ALTER TABLE coaching_ticket_ledger
    ADD COLUMN purchase_ref uuid REFERENCES coaching_ticket_purchases(id);

UPDATE coaching_ticket_ledger e SET purchase_ref = l.purchase_id
FROM coaching_ticket_lots l WHERE e.lot_id = l.id AND e.purchase_ref IS NULL;

CREATE INDEX coaching_ticket_ledger_purchase_ref_idx ON coaching_ticket_ledger(purchase_ref);
