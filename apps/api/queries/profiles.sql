-- name: GetProfile :one
SELECT id, nickname, bio, is_admin, created_at, updated_at
FROM profiles
WHERE id = $1;

-- name: GetProfileByNickname :one
SELECT id, nickname, bio, is_admin, created_at, updated_at
FROM profiles
WHERE nickname = $1;

-- name: UpsertProfile :one
INSERT INTO profiles (id, nickname, bio)
VALUES ($1, $2, $3)
ON CONFLICT (id) DO UPDATE
SET nickname = EXCLUDED.nickname,
    bio = EXCLUDED.bio,
    updated_at = now()
RETURNING id, nickname, bio, is_admin, created_at, updated_at;
