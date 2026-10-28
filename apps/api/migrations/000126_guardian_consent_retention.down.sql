ALTER TABLE guardian_notices DROP COLUMN payment_intent_id;
DROP INDEX guardian_consents_account_deleted_idx;
ALTER TABLE guardian_consents DROP COLUMN account_deleted_at, DROP COLUMN last_transaction_at;
DELETE FROM guardian_notices n WHERE NOT EXISTS (SELECT 1 FROM profiles p WHERE p.id = n.minor_id);
DELETE FROM guardian_consent_events e WHERE NOT EXISTS (SELECT 1 FROM profiles p WHERE p.id = e.minor_id);
DELETE FROM guardian_consents gc WHERE NOT EXISTS (SELECT 1 FROM profiles p WHERE p.id = gc.minor_id);
ALTER TABLE guardian_consents
    ADD CONSTRAINT guardian_consents_minor_id_fkey FOREIGN KEY (minor_id) REFERENCES profiles (id) ON DELETE CASCADE;
ALTER TABLE guardian_consent_events
    ADD CONSTRAINT guardian_consent_events_minor_id_fkey FOREIGN KEY (minor_id) REFERENCES profiles (id) ON DELETE CASCADE;
ALTER TABLE guardian_notices
    ADD CONSTRAINT guardian_notices_minor_id_fkey FOREIGN KEY (minor_id) REFERENCES profiles (id) ON DELETE CASCADE;
