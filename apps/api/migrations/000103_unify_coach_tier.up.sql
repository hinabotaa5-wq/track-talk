-- コーチ区分を一本化する。支払い・購入・報酬の保存済み金額は変更しない。
UPDATE coach_profiles SET tier = 'coach' WHERE tier <> 'coach';
UPDATE review_requests SET tier = 'coach' WHERE tier <> 'coach';
ALTER TABLE coach_profiles DROP CONSTRAINT coach_profiles_tier_check;
ALTER TABLE coach_profiles ADD CONSTRAINT coach_profiles_tier_check CHECK (tier = 'coach');
ALTER TABLE review_requests DROP CONSTRAINT review_requests_tier_check;
ALTER TABLE review_requests ADD CONSTRAINT review_requests_tier_check CHECK (tier = 'coach');
