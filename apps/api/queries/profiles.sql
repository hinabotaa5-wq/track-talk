-- name: GetProfileByID :one
-- ほぼ全リクエストで呼ぶので、既存の行を更新せずに読むだけにする（行ロックや dead tuple を作らない）。
SELECT * FROM profiles WHERE id = $1;

-- name: EnsureProfile :one
-- 退会したアカウントでは作らない（行が返らない）。
INSERT INTO profiles (id, nickname, nickname_public)
SELECT sqlc.arg(id)::uuid, sqlc.narg(nickname)::text, sqlc.arg(nickname_public)::boolean
WHERE NOT EXISTS (SELECT 1 FROM deleted_accounts d WHERE d.id = sqlc.arg(id)::uuid)
ON CONFLICT (id) DO UPDATE
SET updated_at = profiles.updated_at
RETURNING *;

-- name: UpdateProfile :one
UPDATE profiles
SET nickname = $2,
    nickname_public = $3,
    updated_at = now()
WHERE id = $1
RETURNING *;
