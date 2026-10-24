-- 以前のスキーマに戻す。統合済みデータの区分はそのままにする。
ALTER TABLE coach_profiles DROP CONSTRAINT coach_profiles_tier_check;
ALTER TABLE coach_profiles ADD CONSTRAINT coach_profiles_tier_check CHECK (tier IN ('coach', 'senpai'));
ALTER TABLE review_requests DROP CONSTRAINT review_requests_tier_check;
ALTER TABLE review_requests ADD CONSTRAINT review_requests_tier_check CHECK (tier IN ('coach', 'senpai'));
