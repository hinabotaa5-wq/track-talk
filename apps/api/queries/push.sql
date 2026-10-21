-- name: UpsertPushSubscription :one
-- 同じ端末を別のアカウントで登録し直したら、新しいアカウントに付け替える。
INSERT INTO push_subscriptions (id, profile_id, endpoint, p256dh, auth)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (endpoint) DO UPDATE
SET profile_id = EXCLUDED.profile_id,
    p256dh = EXCLUDED.p256dh,
    auth = EXCLUDED.auth,
    created_at = now()
RETURNING *;

-- name: DeletePushSubscription :execrows
DELETE FROM push_subscriptions
WHERE profile_id = $1
  AND endpoint = $2;

-- name: DeletePushSubscriptionByEndpoint :exec
DELETE FROM push_subscriptions
WHERE endpoint = $1;

-- name: ListPushSubscriptions :many
SELECT * FROM push_subscriptions
WHERE profile_id = $1
ORDER BY created_at DESC;

-- name: CountPushSubscriptions :one
SELECT count(*) FROM push_subscriptions
WHERE profile_id = $1;

-- name: ClaimMonthlyReminder :one
-- その月のお知らせを送る権利を取る。すでに送った(送っている)月は行が返らない。
INSERT INTO monthly_reminders (month) VALUES (sqlc.arg(month)::date)
ON CONFLICT (month) DO NOTHING
RETURNING month;

-- name: FinishMonthlyReminder :exec
UPDATE monthly_reminders
SET finished_at = now(), recipients = sqlc.arg(recipients)::int
WHERE month = sqlc.arg(month)::date;

-- name: TrimPushSubscriptions :exec
-- 新しいものから max_devices 台だけ残し、古い端末の登録を消す。
DELETE FROM push_subscriptions
WHERE profile_id = sqlc.arg(profile_id)::uuid
  AND id NOT IN (
    SELECT keep.id FROM push_subscriptions keep
    WHERE keep.profile_id = sqlc.arg(profile_id)::uuid
    ORDER BY keep.created_at DESC, keep.id DESC
    LIMIT sqlc.arg(max_devices)::int
  );

-- name: ReleaseMonthlyReminder :exec
-- 送る前に失敗したとき、その月の権利を返す。送り終えた月(finished_at がある月)は消さない。
DELETE FROM monthly_reminders
WHERE month = sqlc.arg(month)::date
  AND finished_at IS NULL;
