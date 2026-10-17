DROP TABLE coach_earnings;
DROP TABLE coach_payouts;

ALTER TABLE coach_profiles
    DROP COLUMN reduced_fee_until,
    DROP COLUMN payouts_enabled,
    DROP COLUMN stripe_account_id;
