-- name: CreateReport :one
INSERT INTO reports (id, reporter_id, target_type, target_id, reason, detail)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, reporter_id, target_type, target_id, reason, detail, status, created_at;

-- name: ListReports :many
SELECT id, reporter_id, target_type, target_id, reason, detail, status, created_at
FROM reports
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountReports :one
SELECT count(*) FROM reports;

-- name: ListAdminReports :many
SELECT
    r.id,
    r.target_type,
    r.target_id,
    r.reason,
    r.detail,
    r.status,
    r.created_at,
    rp.nickname AS reporter_nickname,
    COALESCE(p.id, c.post_id, '00000000-0000-0000-0000-000000000000'::uuid)::uuid AS post_id,
    COALESCE(p.title, ''::text)::text AS target_title,
    COALESCE(p.body, c.body, ''::text)::text AS target_body,
    (COALESCE(p.hidden_at, c.hidden_at) IS NOT NULL)::boolean AS target_hidden
FROM reports r
JOIN profiles rp ON rp.id = r.reporter_id
LEFT JOIN posts p ON r.target_type = 'post' AND p.id = r.target_id
LEFT JOIN comments c ON r.target_type = 'comment' AND c.id = r.target_id
ORDER BY r.created_at DESC
LIMIT $1 OFFSET $2;
