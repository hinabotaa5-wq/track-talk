-- 失効後の返金（本体 #572 項目7）は1枚あたり単発価格（500円）で返すため、購入額を超えることがある。
-- 上限を「単発価格 × 枚数」とする。単発価格（reviewPriceYen）を変えるときはこの上限も移行する。
ALTER TABLE coaching_ticket_purchases DROP CONSTRAINT coaching_ticket_purchases_check;
ALTER TABLE coaching_ticket_purchases
    ADD CONSTRAINT coaching_ticket_purchases_refund_cap_check
    CHECK (refunded_yen >= 0 AND refunded_yen <= quantity * 500);
