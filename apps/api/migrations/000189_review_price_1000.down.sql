-- 1枚あたり500円を超えて返金済みの購入があると、上限を戻せずに失敗する（その場合は個別に確認する）。
ALTER TABLE coaching_ticket_purchases DROP CONSTRAINT coaching_ticket_purchases_refund_cap_check;
ALTER TABLE coaching_ticket_purchases
    ADD CONSTRAINT coaching_ticket_purchases_refund_cap_check
    CHECK (refunded_yen >= 0 AND refunded_yen <= quantity * 500);

UPDATE coaching_ticket_products SET amount_yen = 2500 WHERE id = 'six';

ALTER TABLE review_requests ALTER COLUMN base_price_yen SET DEFAULT 500;
