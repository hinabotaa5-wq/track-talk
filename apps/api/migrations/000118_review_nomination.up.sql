-- 指名料はコーチが設定し、依頼時に金額と指名先を固定する。
ALTER TABLE coach_profiles ADD COLUMN nomination_fee_yen integer
    CHECK (nomination_fee_yen BETWEEN 0 AND 9998700 AND nomination_fee_yen % 100 = 0);
ALTER TABLE review_requests
    ADD COLUMN requested_coach_id uuid,
    ADD COLUMN requested_coach_name text NOT NULL DEFAULT '',
    ADD COLUMN nomination_fee_yen integer NOT NULL DEFAULT 0 CHECK (nomination_fee_yen >= 0),
    ADD COLUMN base_price_yen integer NOT NULL DEFAULT 1200 CHECK (base_price_yen > 0),
    ADD COLUMN use_tickets boolean NOT NULL DEFAULT false,
    ADD CONSTRAINT review_nomination_fee_requires_coach CHECK (nomination_fee_yen = 0 OR requested_coach_id IS NOT NULL);
UPDATE review_requests r SET use_tickets = true
WHERE EXISTS (SELECT 1 FROM coaching_ticket_reservations t WHERE t.request_id = r.id);
-- 指名先IDは退会後も固定する。別のコーチへの自動振替を防ぐため、外部キーでNULLにしない。
