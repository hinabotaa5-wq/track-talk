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
