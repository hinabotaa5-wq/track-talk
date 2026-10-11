-- name: CreateInquiry :one
INSERT INTO inquiries (id, user_id, category, subject, body)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, user_id, category, subject, body, created_at;

-- name: ListMyInquiries :many
SELECT id, user_id, category, subject, body, created_at
FROM inquiries
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountMyInquiries :one
SELECT count(*) FROM inquiries WHERE user_id = $1;

-- name: ListInquiries :many
SELECT id, user_id, category, subject, body, created_at
FROM inquiries
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountInquiries :one
SELECT count(*) FROM inquiries;
