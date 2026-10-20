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
WHERE token_hash = $1
  AND link_issued_at > now() - interval '30 days';

-- name: IssueGuardianLink :one
-- リンクを出し直すと前のリンクは使えなくなる。リンクは出してから30日で使えなくなる。同意の状態はそのまま残す。
-- 1日（日本時間）に送れるのは daily_limit 回まで。保護者が一度答えたあとは、送り先のメールアドレスを変えられない。
-- どちらかに当たると行を返さない。
INSERT INTO guardian_consents (minor_id, token_hash, guardian_email, link_sent_on, link_sends)
VALUES ($1, $2, $3, (now() AT TIME ZONE 'Asia/Tokyo')::date, 1)
ON CONFLICT (minor_id) DO UPDATE
SET token_hash = EXCLUDED.token_hash,
    guardian_email = EXCLUDED.guardian_email,
    link_sends = CASE WHEN guardian_consents.link_sent_on = EXCLUDED.link_sent_on THEN guardian_consents.link_sends + 1 ELSE 1 END,
    link_sent_on = EXCLUDED.link_sent_on,
    link_issued_at = now(),
    updated_at = now()
WHERE (guardian_consents.link_sent_on IS DISTINCT FROM EXCLUDED.link_sent_on OR guardian_consents.link_sends < sqlc.arg(daily_limit)::integer)
  AND (guardian_consents.status = 'pending' OR lower(guardian_consents.guardian_email) = lower(EXCLUDED.guardian_email))
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
