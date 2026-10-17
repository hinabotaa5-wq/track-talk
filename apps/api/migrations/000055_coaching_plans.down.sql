DROP TABLE coaching_plans;

ALTER TABLE coach_profiles
    DROP CONSTRAINT coach_profiles_pledge_check,
    DROP COLUMN pledge_version,
    DROP COLUMN pledged_at;
