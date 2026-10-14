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

-- name: CountInjuryPostsByPart :many
SELECT i.body_part, count(*)::bigint AS posts
FROM injury_details i
JOIN posts p ON p.id = i.post_id
WHERE p.deleted_at IS NULL
  AND p.hidden_at IS NULL
GROUP BY i.body_part;

-- name: InjuryStageStats :many
-- 部位ごと・段階ごとに、その段階まで経過を報告した投稿の数と、投稿から最初の報告までの日数の中央値。
SELECT
    i.body_part,
    f.stage,
    count(*)::bigint AS reached,
    COALESCE(
        percentile_cont(0.5) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (f.first_at - p.created_at)) / 86400.0),
        0
    )::float8 AS median_days
FROM (
    SELECT post_id, stage, min(created_at) AS first_at
    FROM injury_updates
    GROUP BY post_id, stage
) f
JOIN posts p ON p.id = f.post_id
JOIN injury_details i ON i.post_id = p.id
WHERE p.deleted_at IS NULL
  AND p.hidden_at IS NULL
GROUP BY i.body_part, f.stage;

-- name: ListMyInjuryPosts :many
SELECT p.id, p.title, p.created_at, i.mood, i.body_part
FROM posts p
JOIN injury_details i ON i.post_id = p.id
WHERE p.author_id = $1
  AND p.deleted_at IS NULL
  AND p.hidden_at IS NULL
ORDER BY p.created_at DESC;

-- name: ListMyInjuryFirstReports :many
SELECT u.post_id, u.stage, min(u.created_at)::timestamptz AS first_at
FROM injury_updates u
JOIN posts p ON p.id = u.post_id
WHERE p.author_id = $1
  AND p.deleted_at IS NULL
  AND p.hidden_at IS NULL
GROUP BY u.post_id, u.stage;
