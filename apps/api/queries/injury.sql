-- name: UpsertInjuryDetail :exec
INSERT INTO injury_details (post_id, mood, body_part)
VALUES ($1, $2, $3)
ON CONFLICT (post_id) DO UPDATE SET
    mood = EXCLUDED.mood,
    body_part = EXCLUDED.body_part;

-- name: GetInjuryDetail :one
SELECT post_id, mood, body_part
FROM injury_details
WHERE post_id = $1;

-- name: ListInjuryDetails :many
SELECT post_id, mood, body_part
FROM injury_details
WHERE post_id = ANY(sqlc.arg('post_ids')::uuid[]);

-- name: CreateInjuryUpdate :one
INSERT INTO injury_updates (id, post_id, stage, body)
VALUES ($1, $2, $3, $4)
RETURNING id, post_id, stage, body, created_at;

-- name: ListInjuryUpdates :many
SELECT id, post_id, stage, body, created_at
FROM injury_updates
WHERE post_id = $1
ORDER BY created_at ASC;

-- name: CountInjuryUpdates :one
SELECT count(*) FROM injury_updates WHERE post_id = $1;

-- name: DeleteInjuryUpdate :execrows
DELETE FROM injury_updates u
USING posts p
WHERE u.id = $1
  AND u.post_id = $2
  AND p.id = u.post_id
  AND p.author_id = $3;

-- name: ListLatestInjuryStages :many
SELECT DISTINCT ON (post_id) post_id, stage
FROM injury_updates
WHERE post_id = ANY(sqlc.arg('post_ids')::uuid[])
ORDER BY post_id, created_at DESC;
