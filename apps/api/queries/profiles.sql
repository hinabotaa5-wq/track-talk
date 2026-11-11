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
-- locale は送らなければそのまま、送ったら ja / en だけ受け付ける（判定はハンドラ側）。
UPDATE profiles
SET nickname = $2,
    nickname_public = $3,
    event = COALESCE(sqlc.narg(event)::text, event),
    locale = COALESCE(sqlc.narg(locale)::text, locale),
    updated_at = now()
WHERE id = $1
  AND (NOT sqlc.arg(reserved_nickname)::boolean OR is_admin OR nickname = $2)
RETURNING *;

-- name: UpdateProfileLocale :one
-- 表示言語だけを変える（本体 #535）。ニックネームなど他の項目には触れない。
UPDATE profiles
SET locale = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdatePostMasterBadge :one
UPDATE profiles
SET post_master_badge_public = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;
