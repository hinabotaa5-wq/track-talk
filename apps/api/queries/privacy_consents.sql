-- name: HasPrivacyConsent :one
SELECT EXISTS (
    SELECT 1 FROM privacy_consents WHERE user_id = $1 AND kind = $2 AND version = $3
);

-- name: RecordPrivacyConsent :exec
INSERT INTO privacy_consents (user_id, kind, version)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, kind) DO UPDATE
SET version = EXCLUDED.version, consented_at = now();
