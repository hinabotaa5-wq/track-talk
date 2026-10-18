ALTER TABLE guardian_consents
    DROP CONSTRAINT guardian_consents_guardian_email_check,
    DROP COLUMN link_sends,
    DROP COLUMN link_sent_on,
    DROP COLUMN guardian_email;
