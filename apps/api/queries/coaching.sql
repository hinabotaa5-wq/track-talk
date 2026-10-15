-- name: SetBirthMonth :one
-- 生まれた年月は一度だけ登録できる。18歳未満が大人に書き換えるのを防ぐため、変更は問い合わせで受ける。
UPDATE profiles
SET birth_month = $2,
    updated_at = now()
WHERE id = $1
  AND birth_month IS NULL
RETURNING *;

-- name: GetGuardianConsentByMinor :one
SELECT * FROM guardian_consents
WHERE minor_id = $1;

-- name: GetGuardianConsentByToken :one
SELECT * FROM guardian_consents
WHERE token_hash = $1;

-- name: IssueGuardianLink :one
-- リンクを出し直すと前のリンクは使えなくなる。同意の状態はそのまま残す。
INSERT INTO guardian_consents (minor_id, token_hash)
VALUES ($1, $2)
ON CONFLICT (minor_id) DO UPDATE
SET token_hash = EXCLUDED.token_hash,
    updated_at = now()
RETURNING *;

-- name: ConsentGuardian :one
UPDATE guardian_consents
SET status = 'consented',
    guardian_name = $2,
    relationship = $3,
    monthly_limit_yen = $4,
    allow_publish = $5,
    consented_at = now(),
    declined_at = NULL,
    revoked_at = NULL,
    updated_at = now()
WHERE token_hash = $1
RETURNING *;

-- name: UpdateGuardianSettings :one
UPDATE guardian_consents
SET monthly_limit_yen = $2,
    allow_publish = $3,
    updated_at = now()
WHERE token_hash = $1
  AND status = 'consented'
RETURNING *;

-- name: DeclineGuardian :one
UPDATE guardian_consents
SET status = 'declined',
    declined_at = now(),
    updated_at = now()
WHERE token_hash = $1
  AND status IN ('pending', 'declined')
RETURNING *;

-- name: RevokeGuardian :one
UPDATE guardian_consents
SET status = 'revoked',
    revoked_at = now(),
    updated_at = now()
WHERE token_hash = $1
  AND status = 'consented'
RETURNING *;
