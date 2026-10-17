DROP TRIGGER coaching_access_logs_immutable ON coaching_access_logs;
DROP FUNCTION coaching_access_logs_immutable();
DROP TABLE coaching_access_logs;

DELETE FROM reports WHERE target_type = 'coaching_message';
ALTER TABLE reports
    DROP CONSTRAINT reports_reporter_target_key,
    DROP CONSTRAINT reports_by_guardian_check,
    DROP CONSTRAINT reports_target_type_check,
    DROP COLUMN by_guardian,
    ADD CONSTRAINT reports_target_type_check CHECK (target_type IN ('post', 'comment', 'injury_note', 'review_response')),
    ADD CONSTRAINT reports_reporter_id_target_type_target_id_key UNIQUE (reporter_id, target_type, target_id);

DELETE FROM coaching_contract_consents WHERE party = 'guardian';
ALTER TABLE coaching_contract_consents
    DROP CONSTRAINT coaching_contract_consents_profile_check,
    DROP CONSTRAINT coaching_contract_consents_party_check,
    ADD CONSTRAINT coaching_contract_consents_party_check CHECK (party IN ('requester', 'coach')),
    ALTER COLUMN profile_id SET NOT NULL;

DROP INDEX coaching_contracts_plan_idx;
CREATE INDEX coaching_contracts_plan_idx ON coaching_contracts (plan_id) WHERE status IN ('applied', 'active');
DROP INDEX coaching_contracts_open_idx;
CREATE UNIQUE INDEX coaching_contracts_open_idx ON coaching_contracts (requester_id, coach_id) WHERE status IN ('applied', 'active');

UPDATE coaching_contracts SET status = 'cancelled' WHERE status IN ('accepted', 'guardian_declined');
UPDATE coaching_contracts SET status = 'ended' WHERE status = 'suspended';
ALTER TABLE coaching_contracts
    DROP CONSTRAINT coaching_contracts_suspended_check,
    DROP CONSTRAINT coaching_contracts_start_by_check,
    DROP CONSTRAINT coaching_contracts_period_check,
    DROP CONSTRAINT coaching_contracts_status_check,
    DROP COLUMN suspended_at,
    DROP COLUMN start_by,
    ADD CONSTRAINT coaching_contracts_status_check CHECK (status IN ('applied', 'active', 'ended', 'declined', 'expired', 'cancelled')),
    ADD CONSTRAINT coaching_contracts_check1 CHECK (status NOT IN ('active', 'ended') OR (starts_at IS NOT NULL AND ends_at IS NOT NULL));
