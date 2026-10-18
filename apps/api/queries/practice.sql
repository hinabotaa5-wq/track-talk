-- name: UpsertPracticeDetail :exec
INSERT INTO practice_details (
    post_id, venue, starts_at, event, event_other, capacity
) VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (post_id) DO UPDATE SET
    venue = EXCLUDED.venue,
    starts_at = EXCLUDED.starts_at,
    event = EXCLUDED.event,
    event_other = EXCLUDED.event_other,
    capacity = EXCLUDED.capacity;

-- name: GetPracticeDetail :one
SELECT post_id, venue, starts_at, event, event_other, capacity, dissolved_at
FROM practice_details
WHERE post_id = $1;

-- name: ListPracticeDetails :many
SELECT post_id, venue, starts_at, event, event_other, capacity, dissolved_at
FROM practice_details
WHERE post_id = ANY(sqlc.arg('post_ids')::uuid[]);

-- name: LockPracticeDetail :one
SELECT post_id, venue, starts_at, event, event_other, capacity, dissolved_at
FROM practice_details
WHERE post_id = $1
FOR UPDATE;

-- name: DissolvePractice :execrows
UPDATE practice_details
SET dissolved_at = now()
WHERE post_id = $1
  AND dissolved_at IS NULL;

-- name: CountAcceptedApplications :one
SELECT count(*)::int
FROM practice_applications
WHERE post_id = $1
  AND status = 'accepted';

-- name: ListAcceptedCounts :many
SELECT post_id, count(*)::int AS accepted_count
FROM practice_applications
WHERE status = 'accepted'
  AND post_id = ANY(sqlc.arg('post_ids')::uuid[])
GROUP BY post_id;

-- name: GetPracticeApplication :one
SELECT id, post_id, user_id, status, created_at
FROM practice_applications
WHERE id = $1
  AND post_id = $2;

-- name: GetPracticeApplicationByUser :one
SELECT id, post_id, user_id, status, created_at
FROM practice_applications
WHERE post_id = $1
  AND user_id = $2;

-- name: ListPracticeApplications :many
SELECT id, user_id, status, created_at
FROM practice_applications
WHERE post_id = $1
ORDER BY created_at;

-- name: ListAcceptedPracticeApplications :many
SELECT id, user_id
FROM practice_applications
WHERE post_id = $1
  AND status = 'accepted'
ORDER BY created_at;

-- name: InsertPracticeApplication :exec
INSERT INTO practice_applications (id, post_id, user_id, status)
VALUES ($1, $2, $3, 'pending');

-- name: UpdatePracticeApplicationStatus :execrows
UPDATE practice_applications
SET status = $3,
    updated_at = now()
WHERE id = $1
  AND post_id = $2
  AND status = 'pending';

-- name: GetPracticeRatingByRater :one
SELECT id
FROM practice_ratings
WHERE application_id = $1
  AND rater_id = $2;

-- name: ListRatedApplicationIDs :many
SELECT application_id
FROM practice_ratings
WHERE post_id = $1
  AND rater_id = $2;

-- name: InsertPracticeRating :exec
INSERT INTO practice_ratings (
    id, post_id, application_id, rater_id, ratee_id, score, comment
) VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: PracticeReputationCounts :one
SELECT
    (count(*) FILTER (WHERE score = 'good'))::int AS good,
    (count(*) FILTER (WHERE score = 'ok'))::int AS ok,
    (count(*) FILTER (WHERE score = 'bad'))::int AS bad
FROM practice_ratings
WHERE ratee_id = $1;

-- name: ListPracticeReputationCounts :many
SELECT
    ratee_id,
    (count(*) FILTER (WHERE score = 'good'))::int AS good,
    (count(*) FILTER (WHERE score = 'ok'))::int AS ok,
    (count(*) FILTER (WHERE score = 'bad'))::int AS bad
FROM practice_ratings
WHERE ratee_id = ANY(sqlc.arg('user_ids')::uuid[])
GROUP BY ratee_id;

-- name: ListPracticeReputationComments :many
SELECT score, comment, created_at
FROM practice_ratings
WHERE ratee_id = $1
  AND comment <> ''
ORDER BY created_at DESC
LIMIT 5;

-- 参加者として主催者から評価された回数と、そのうちドタキャンだった回数。
-- name: ListApplicantAttendanceCounts :many
SELECT
    a.user_id,
    count(*)::int AS total,
    (count(*) FILTER (WHERE r.score = 'no_show'))::int AS no_show
FROM practice_ratings r
JOIN practice_applications a ON a.id = r.application_id
WHERE r.ratee_id = a.user_id
  AND a.user_id = ANY(sqlc.arg('user_ids')::uuid[])
GROUP BY a.user_id;
