-- name: StartCoachStripeAccountAttempt :exec
INSERT INTO coach_stripe_account_attempts (profile_id, idempotency_key, started_at)
VALUES (sqlc.arg(profile_id), sqlc.arg(idempotency_key), clock_timestamp())
ON CONFLICT (profile_id) DO UPDATE SET started_at = clock_timestamp()
WHERE coach_stripe_account_attempts.started_at IS NULL;

-- name: LockCoachStripeAccountAttempt :one
SELECT c.stripe_account_id, a.idempotency_key, a.started_at
FROM coach_profiles c
JOIN coach_stripe_account_attempts a USING (profile_id)
WHERE c.profile_id = sqlc.arg(profile_id)
FOR UPDATE OF c, a;

-- name: RotateCoachStripeAccountAttempt :exec
UPDATE coach_stripe_account_attempts
SET idempotency_key = sqlc.arg(idempotency_key), started_at = NULL
WHERE profile_id = sqlc.arg(profile_id);
