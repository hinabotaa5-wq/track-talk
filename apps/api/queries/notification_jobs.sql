-- name: EnqueueNotificationJob :exec
INSERT INTO notification_jobs (id, kind, recipient_id, post_id, comment_id, application_id, review_request_id, reviewed_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: ListDueNotificationJobs :many
SELECT id FROM notification_jobs
WHERE next_attempt_at <= now()
ORDER BY next_attempt_at, id
LIMIT 100;

-- name: LockNotificationJob :one
SELECT * FROM notification_jobs WHERE id = $1 FOR UPDATE SKIP LOCKED;

-- name: DeleteNotificationJob :exec
DELETE FROM notification_jobs WHERE id = $1;

-- name: DeferNotificationJob :exec
UPDATE notification_jobs SET next_attempt_at = now() + interval '1 minute'
WHERE id = $1;

-- name: GetRejectionNotificationJob :one
SELECT id FROM notification_jobs
WHERE recipient_id = $1 AND reviewed_at = $2 AND kind = 'coach_rejected';

-- name: LockRejectedCoachApplication :one
SELECT profile_id FROM coach_profiles
WHERE profile_id = $1 AND reviewed_at = $2 AND status = 'rejected'
FOR UPDATE;

-- name: InsertRejectionNotification :exec
INSERT INTO notifications (id, recipient_id, kind, title, body, url)
VALUES ($1, $2, 'message', $3, $4, $5);
