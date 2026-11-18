-- name: RecordDocumentConsents :exec
-- One statement records the complete displayed set atomically; retries preserve the first timestamp.
INSERT INTO document_consents (user_id, document, version)
VALUES (sqlc.arg(user_id)::uuid, 'terms', sqlc.arg(terms_version)::text),
       (sqlc.arg(user_id)::uuid, 'guidelines', sqlc.arg(guidelines_version)::text),
       (sqlc.arg(user_id)::uuid, 'privacy', sqlc.arg(privacy_version)::text)
ON CONFLICT (user_id, document, version) DO NOTHING;

-- name: HasDocumentConsents :one
SELECT count(*) = 3 FROM document_consents
WHERE user_id = sqlc.arg(user_id)::uuid
  AND ((document = 'terms' AND version = sqlc.arg(terms_version)::text)
    OR (document = 'guidelines' AND version = sqlc.arg(guidelines_version)::text)
    OR (document = 'privacy' AND version = sqlc.arg(privacy_version)::text));

-- name: KeepDocumentConsentsOnWithdraw :exec
-- Same transaction sources and timestamp as KeepGuardianRecordsOnWithdraw, before profiles/payments lose their user.
UPDATE document_consents dc SET account_deleted_at = now(),
    last_transaction_at = (
        SELECT max(t) FROM (
            SELECT max(GREATEST(p.captured_at, p.refunded_at)) AS t FROM review_payments p
            WHERE p.payer_id = dc.user_id AND p.status IN ('captured', 'refunded')
            UNION ALL
            SELECT max(GREATEST(c.paid_at, CASE WHEN c.payment_state = 'refunded' THEN c.updated_at END)) FROM coaching_consultations c
            WHERE c.requester_id = dc.user_id AND c.contract_id IS NULL AND c.payment_state <> 'unpaid'
            UNION ALL
            SELECT max(n.purchased_at) FROM guardian_notices n
            WHERE n.minor_id = dc.user_id AND n.kind = 'purchase'
        ) x
    )
WHERE dc.user_id = sqlc.arg(user_id)::uuid;

-- name: DeleteExpiredDocumentConsents :execrows
DELETE FROM document_consents
WHERE account_deleted_at IS NOT NULL
  AND COALESCE(last_transaction_at, consented_at) < sqlc.arg(cutoff)::timestamptz;
