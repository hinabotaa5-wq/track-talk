-- name: CreateInquiry :one
INSERT INTO inquiries (id, user_id, category, subject, body, source)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, user_id, category, subject, body, source, created_at;

-- name: ListMyInquiries :many
SELECT id, user_id, category, subject, body, source, created_at
FROM inquiries
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountMyInquiries :one
SELECT count(*) FROM inquiries WHERE user_id = $1;

-- name: ListInquiries :many
SELECT id, user_id, category, subject, body, source, created_at
FROM inquiries
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountInquiries :one
SELECT count(*) FROM inquiries;

-- name: ListAdminInquiries :many
SELECT i.id, i.category, i.subject, i.body, i.source, i.created_at, p.nickname AS user_nickname
FROM inquiries i
JOIN profiles p ON p.id = i.user_id
WHERE i.source = $1
ORDER BY i.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountAdminInquiries :one
SELECT count(*) FROM inquiries WHERE source = $1;
