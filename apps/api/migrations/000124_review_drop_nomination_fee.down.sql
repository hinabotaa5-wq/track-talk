-- 列は戻すが、消した指名料の値は戻らない(すべて未設定・0円になる)。
ALTER TABLE coach_profiles ADD COLUMN nomination_fee_yen integer
    CHECK (nomination_fee_yen BETWEEN 0 AND 9998700 AND nomination_fee_yen % 100 = 0);
ALTER TABLE review_requests ALTER COLUMN base_price_yen SET DEFAULT 1200;
ALTER TABLE review_requests ADD COLUMN nomination_fee_yen integer NOT NULL DEFAULT 0 CHECK (nomination_fee_yen >= 0),
    ADD CONSTRAINT review_nomination_fee_requires_coach CHECK (nomination_fee_yen = 0 OR requested_coach_id IS NOT NULL);
