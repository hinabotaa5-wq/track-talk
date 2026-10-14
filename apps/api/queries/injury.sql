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
