-- name: EnsureProfile :one
INSERT INTO profiles (id)
VALUES ($1)
ON CONFLICT (id) DO UPDATE
SET updated_at = profiles.updated_at
RETURNING id, is_admin, created_at, updated_at;
