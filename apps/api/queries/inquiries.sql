-- name: CreateInquiry :one
INSERT INTO inquiries (id, user_id, category, subject, body, source, contact_email)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, user_id, category, subject, body, source, contact_email, created_at;

-- name: ListMyInquiries :many
SELECT id, user_id, category, subject, body, source, contact_email, created_at,
    responded_at, response_body
FROM inquiries
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountMyInquiries :one
SELECT count(*) FROM inquiries WHERE user_id = $1;

-- name: ListAdminInquiries :many
SELECT i.id, i.category, i.subject, i.body, i.source, i.contact_email, i.created_at,
    i.responded_at, i.response_body, i.responder_id, p.nickname AS user_nickname
FROM inquiries i
JOIN profiles p ON p.id = i.user_id
WHERE i.source = $1
ORDER BY i.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountAdminInquiries :one
SELECT count(*) FROM inquiries WHERE source = $1;

-- name: RecordInquiryResponse :one
UPDATE inquiries
SET responded_at = now(), response_body = $2, responder_id = $3
WHERE id = $1
RETURNING id, responded_at, response_body, responder_id;
