DROP INDEX coach_legal_identities_ended_idx;
ALTER TABLE coach_legal_identities DROP COLUMN coaching_ended_at, DROP COLUMN last_transaction_at;
DELETE FROM coach_legal_identities li WHERE NOT EXISTS (SELECT 1 FROM coach_profiles cp WHERE cp.profile_id = li.coach_id);
ALTER TABLE coach_legal_identities
    ADD CONSTRAINT coach_legal_identities_coach_id_fkey FOREIGN KEY (coach_id) REFERENCES coach_profiles (profile_id) ON DELETE CASCADE;
