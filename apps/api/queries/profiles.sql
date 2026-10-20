-- name: EnsureProfile :one
INSERT INTO profiles (id, nickname, nickname_public)
VALUES ($1, $2, $3)
ON CONFLICT (id) DO UPDATE
SET updated_at = profiles.updated_at
RETURNING *;

-- name: UpdateProfile :one
UPDATE profiles
SET nickname = $2,
    nickname_public = $3,
    post_master_badge_public = $4,
    updated_at = now()
WHERE id = $1
RETURNING *;
