DROP TABLE push_subscriptions;
DROP TABLE guardian_consents;
ALTER TABLE profiles
    DROP CONSTRAINT profiles_birth_month_check,
    DROP COLUMN birth_month;
