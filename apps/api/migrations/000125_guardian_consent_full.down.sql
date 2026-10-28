DROP TABLE IF EXISTS guardian_notices;
DROP TABLE IF EXISTS guardian_manage_links;
DROP TABLE IF EXISTS guardian_consent_events;
ALTER TABLE guardian_consents
    DROP COLUMN IF EXISTS self_pay_limit_yen,
    DROP COLUMN IF EXISTS consent_ip,
    DROP COLUMN IF EXISTS terms_version,
    DROP COLUMN IF EXISTS consent_token_hash,
    DROP COLUMN IF EXISTS limit_request_sent_at;
