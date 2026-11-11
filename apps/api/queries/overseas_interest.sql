-- name: GetOverseasInterest :one
SELECT kind, created_at FROM overseas_interests WHERE profile_id = $1;

-- name: UpsertOverseasInterest :one
INSERT INTO overseas_interests (profile_id, kind)
VALUES (sqlc.arg(profile_id), sqlc.arg(kind))
ON CONFLICT (profile_id) DO UPDATE
SET kind = EXCLUDED.kind,
    updated_at = now()
RETURNING kind, created_at;

-- name: DeleteOverseasInterest :exec
DELETE FROM overseas_interests WHERE profile_id = $1;

-- name: AdminOverseasInterestStats :one
SELECT COUNT(*)::bigint AS total,
    COUNT(*) FILTER (WHERE kind = 'monthly')::bigint AS monthly,
    COUNT(*) FILTER (WHERE kind = 'single')::bigint AS single,
    COUNT(*) FILTER (WHERE kind = 'both')::bigint AS both
FROM overseas_interests;

-- name: AdminOverseasInterestList :many
SELECT i.kind, i.created_at, COALESCE(NULLIF(p.nickname, ''), '')::text AS nickname
FROM overseas_interests i
JOIN profiles p ON p.id = i.profile_id
ORDER BY i.created_at;
