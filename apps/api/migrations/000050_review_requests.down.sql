DROP TABLE review_requests;
DROP TABLE coaching_demo_codes;
ALTER TABLE profiles DROP COLUMN coaching_demo;
ALTER TABLE coach_profiles
    DROP CONSTRAINT coach_profiles_on_duty_check,
    DROP CONSTRAINT coach_profiles_daily_cap_check,
    DROP COLUMN last_offered_at,
    DROP COLUMN daily_cap,
    DROP COLUMN on_duty;
