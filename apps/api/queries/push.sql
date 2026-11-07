-- name: UpsertPushSubscription :one
-- 同じ端末を別のアカウントで登録し直したら、新しいアカウントに付け替える。
INSERT INTO push_subscriptions (id, profile_id, endpoint, p256dh, auth)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (endpoint) DO UPDATE
SET id = EXCLUDED.id,
    profile_id = EXCLUDED.profile_id,
    p256dh = EXCLUDED.p256dh,
    auth = EXCLUDED.auth,
    created_at = now()
RETURNING *;

-- name: DeletePushSubscription :execrows
DELETE FROM push_subscriptions
WHERE profile_id = $1
  AND endpoint = $2;

-- name: DeletePushSubscriptionGeneration :exec
DELETE FROM push_subscriptions
WHERE id = $1 AND profile_id = $2;

-- name: PushSubscriptionIsCurrent :one
SELECT EXISTS (SELECT 1 FROM push_subscriptions WHERE id = $1 AND profile_id = $2);

-- name: ListPushSubscriptions :many
SELECT * FROM push_subscriptions
WHERE profile_id = $1
ORDER BY created_at DESC;

-- name: CountPushSubscriptions :one
SELECT count(*) FROM push_subscriptions
WHERE profile_id = $1;

-- name: ClaimMonthlyReminder :one
-- その月のお知らせを送る権利を取る。すでに送った月と、いま送っている最中の月は行が返らない。
-- 送り始めてから締め切り（30分＋余裕）を過ぎても終わっていない月は、止まったものとして取り直す。
INSERT INTO monthly_reminders (month) VALUES (sqlc.arg(month)::date)
ON CONFLICT (month) DO UPDATE
SET started_at = now()
WHERE monthly_reminders.finished_at IS NULL
  AND monthly_reminders.started_at < now() - interval '35 minutes'
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

-- name: MarkMonthlyReminderInAppDone :exec
-- ベルに入れ終えたことを残す。呼び直したときにベルを二重に入れない。
UPDATE monthly_reminders
SET in_app_done_at = now()
WHERE month = sqlc.arg(month)::date
  AND in_app_done_at IS NULL;

-- name: GetMonthlyReminder :one
SELECT month, started_at, finished_at, recipients, in_app_done_at FROM monthly_reminders
WHERE month = sqlc.arg(month)::date;

-- name: ListStaleMonthlyReminders :many
-- 送り始めたまま止まっている月。次の実行で未送信分を送り直す。
SELECT month FROM monthly_reminders
WHERE finished_at IS NULL
  AND started_at < now() - interval '35 minutes'
ORDER BY month;

-- name: MarkPushEverSubscribed :exec
-- このアカウントで購読の登録が成功したことを残す。許可が残ったまま購読だけが消えた端末の自動再購読に使う。
UPDATE profiles
SET push_ever_subscribed = true, updated_at = now()
WHERE id = $1
  AND NOT push_ever_subscribed;
