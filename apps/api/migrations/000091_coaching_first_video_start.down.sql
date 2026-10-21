ALTER TABLE coaching_contracts
    DROP CONSTRAINT coaching_contracts_clock_check,
    DROP COLUMN clock_started_at,
    DROP COLUMN start_on_first_video;

ALTER TABLE coaching_plans
    DROP COLUMN start_on_first_video;
