-- 動画添削はコーチを選ぶ方式だけにし、料金は全コーチ共通(1,500円)にする。コーチごとの指名料はなくすので列ごと消す。
-- 選んだコーチの記録(requested_coach_id・requested_coach_name)は、選ぶ方式でそのまま使うので残す。
ALTER TABLE review_requests DROP CONSTRAINT IF EXISTS review_nomination_fee_requires_coach;
ALTER TABLE review_requests DROP COLUMN nomination_fee_yen;
ALTER TABLE review_requests ALTER COLUMN base_price_yen SET DEFAULT 1500;
ALTER TABLE coach_profiles DROP COLUMN nomination_fee_yen;
