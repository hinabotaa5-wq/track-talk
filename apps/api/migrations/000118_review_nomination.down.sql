ALTER TABLE review_requests DROP COLUMN use_tickets, DROP COLUMN base_price_yen,
    DROP COLUMN nomination_fee_yen, DROP COLUMN requested_coach_name, DROP COLUMN requested_coach_id;
ALTER TABLE coach_profiles DROP COLUMN nomination_fee_yen;
