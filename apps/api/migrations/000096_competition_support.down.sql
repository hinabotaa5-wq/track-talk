DROP TABLE coaching_calls,coaching_video_reviews;
ALTER TABLE coaching_messages DROP COLUMN review_submission_id;
ALTER TABLE coaching_contracts DROP COLUMN competition_name,DROP COLUMN competition_date,DROP COLUMN support_start_date,DROP COLUMN call_count,DROP COLUMN call_minutes,DROP COLUMN fulfillment_version,DROP COLUMN fulfillment_state,DROP COLUMN support_refund_amount_yen,DROP COLUMN support_refund_attempt_id,DROP COLUMN support_refund_reason;
-- 大会プランの独自日数は旧形式の表示日数に戻し、非公開にする。
UPDATE coaching_plans SET duration_days=30,published=false WHERE service_type='competition_support';
ALTER TABLE coaching_plans DROP CONSTRAINT support_plan_dates_check;
ALTER TABLE coaching_plans DROP CONSTRAINT coaching_plans_check;
ALTER TABLE coaching_plans ADD CONSTRAINT coaching_plans_check CHECK ((kind='period' AND duration_days IN (14,30)) OR (kind IN ('single','trial') AND duration_days=7));
ALTER TABLE coaching_plans DROP COLUMN competition_name,DROP COLUMN competition_date,DROP COLUMN support_start_date,DROP COLUMN call_count,DROP COLUMN call_minutes;
