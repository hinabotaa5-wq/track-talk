-- 失効した枚数は available には戻さない（戻すと失効を取り消すことになる）。失効の記録がある間は戻せない。
ALTER TABLE minor_purchase_declarations DROP CONSTRAINT minor_purchase_declarations_ref_type_check;
ALTER TABLE minor_purchase_declarations ADD CONSTRAINT minor_purchase_declarations_ref_type_check
    CHECK (ref_type IN ('review', 'consultation'));

ALTER TABLE coaching_ticket_ledger DROP CONSTRAINT coaching_ticket_ledger_kind_check;
ALTER TABLE coaching_ticket_ledger ADD CONSTRAINT coaching_ticket_ledger_kind_check
    CHECK (kind IN ('purchase', 'reserve', 'consume', 'release', 'refund', 'refund_failed'));
ALTER TABLE coaching_ticket_ledger DROP COLUMN expired_delta;

DROP INDEX coaching_ticket_lots_expiry_idx;
ALTER TABLE coaching_ticket_lots DROP CONSTRAINT coaching_ticket_lots_balance_check;
ALTER TABLE coaching_ticket_lots ADD CONSTRAINT coaching_ticket_lots_check
    CHECK (quantity = available + reserved + consumed + refunded);
ALTER TABLE coaching_ticket_lots DROP COLUMN expired, DROP COLUMN expires_at;

UPDATE coaching_ticket_products SET active = false WHERE id = 'six';
UPDATE coaching_ticket_products SET active = true WHERE id IN ('single', 'five');

ALTER TABLE review_requests ALTER COLUMN base_price_yen SET DEFAULT 1500;
