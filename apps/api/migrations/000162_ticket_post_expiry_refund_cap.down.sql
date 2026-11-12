ALTER TABLE coaching_ticket_purchases DROP CONSTRAINT coaching_ticket_purchases_refund_cap_check;
ALTER TABLE coaching_ticket_purchases
    ADD CONSTRAINT coaching_ticket_purchases_check
    CHECK (refunded_yen >= 0 AND refunded_yen <= amount_yen);
