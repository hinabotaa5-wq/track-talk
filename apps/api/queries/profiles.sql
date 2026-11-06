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
-- 予約語の例外は書き込み時点の管理者権限と保存値で判定する。
UPDATE profiles
SET nickname = $2,
    nickname_public = $3,
    event = COALESCE(sqlc.narg(event)::text, event),
    updated_at = now()
WHERE id = $1
  AND (NOT sqlc.arg(reserved_nickname)::boolean OR is_admin OR nickname = $2)
RETURNING *;

-- name: UpdatePostMasterBadge :one
UPDATE profiles
SET post_master_badge_public = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;
