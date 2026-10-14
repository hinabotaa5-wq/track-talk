-- name: UpsertInjuryDetail :exec
INSERT INTO injury_details (post_id, body_part, group_stage)
VALUES ($1, $2, sqlc.narg('group_stage'))
ON CONFLICT (post_id) DO UPDATE SET
    body_part = EXCLUDED.body_part;

-- name: GetInjuryDetail :one
SELECT post_id, body_part, group_stage
FROM injury_details
WHERE post_id = $1;

-- name: ListInjuryDetails :many
SELECT post_id, body_part, group_stage
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
SELECT p.id, p.title, p.created_at, i.body_part
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

-- name: NotifyInjuryFollowers :exec
-- 経過報告があったとき、その投稿に「わかる」か保存をした人(投稿者を除く)へ通知する。
-- 同じ投稿の未読の通知がすでにある人には重ねて送らない。
INSERT INTO notifications (id, recipient_id, kind, post_id)
SELECT gen_random_uuid(), f.user_id, 'injury_update', sqlc.arg('post_id')::uuid
FROM (
    SELECT l.user_id FROM post_likes l WHERE l.post_id = sqlc.arg('post_id')::uuid
    UNION
    SELECT s.user_id FROM post_saves s WHERE s.post_id = sqlc.arg('post_id')::uuid
) f
WHERE f.user_id <> sqlc.arg('author_id')::uuid
  AND NOT EXISTS (
    SELECT 1 FROM notifications n
    WHERE n.recipient_id = f.user_id
      AND n.post_id = sqlc.arg('post_id')::uuid
      AND n.kind = 'injury_update'
      AND n.read_at IS NULL
  );

-- name: UpdateInjuryUpdate :one
UPDATE injury_updates u
SET stage = sqlc.arg('stage'),
    body = sqlc.arg('body')
FROM posts p
WHERE u.id = sqlc.arg('id')
  AND u.post_id = sqlc.arg('post_id')
  AND p.id = u.post_id
  AND p.author_id = sqlc.arg('author_id')
RETURNING u.id, u.post_id, u.stage, u.body, u.created_at;

-- name: UpsertInjuryGroupMember :one
INSERT INTO injury_group_members (user_id, body_part, injured_on, place, situation, stage)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (user_id, body_part) DO UPDATE SET
    injured_on = EXCLUDED.injured_on,
    place = EXCLUDED.place,
    situation = EXCLUDED.situation,
    stage = EXCLUDED.stage,
    updated_at = now()
RETURNING user_id, body_part, injured_on, place, situation, stage, joined_at, updated_at;

-- name: GetInjuryGroupMember :one
SELECT user_id, body_part, injured_on, place, situation, stage, joined_at, updated_at
FROM injury_group_members
WHERE user_id = $1 AND body_part = $2;

-- name: ListMyInjuryGroupMembers :many
SELECT user_id, body_part, injured_on, place, situation, stage, joined_at, updated_at
FROM injury_group_members
WHERE user_id = $1;

-- name: SetInjuryGroupStage :one
UPDATE injury_group_members
SET stage = $3, updated_at = now()
WHERE user_id = $1 AND body_part = $2
RETURNING user_id, body_part, injured_on, place, situation, stage, joined_at, updated_at;

-- name: AddInjuryGroupStageLog :exec
INSERT INTO injury_group_stage_log (user_id, body_part, stage) VALUES ($1, $2, $3);

-- name: CountInjuryGroupMembers :many
SELECT body_part, count(*)::bigint AS members
FROM injury_group_members
GROUP BY body_part;

-- name: CountInjuryGroupMembersByStage :many
SELECT stage, count(*)::bigint AS members
FROM injury_group_members
WHERE body_part = $1
GROUP BY stage;

-- name: ListInjuryGroupFaces :many
-- 段階ごとに、新しく入った順で最大 limit_n 人のニックネーム。
SELECT stage, COALESCE(nickname, '')::text AS nickname
FROM (
    SELECT m.stage, p.nickname,
           row_number() OVER (PARTITION BY m.stage ORDER BY m.updated_at DESC) AS rn
    FROM injury_group_members m
    JOIN profiles p ON p.id = m.user_id
    WHERE m.body_part = $1
) t
WHERE rn <= sqlc.arg('limit_n')::int
ORDER BY stage, rn;

-- name: IsProfileAdmin :one
SELECT COALESCE((SELECT is_admin FROM profiles WHERE id = $1), false)::boolean AS is_admin;

-- name: DeleteInjuryGroupMember :execrows
DELETE FROM injury_group_members WHERE user_id = $1 AND body_part = $2;
