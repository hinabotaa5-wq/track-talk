-- name: InsertNotification :exec
INSERT INTO notifications (id, recipient_id, kind, post_id, comment_id, application_id)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: InsertReviewNotification :exec
INSERT INTO notifications (id, recipient_id, kind, review_request_id)
VALUES ($1, $2, $3, $4);

-- name: InsertRecruitmentPostNotifications :exec
INSERT INTO notifications (id, recipient_id, kind, post_id)
SELECT gen_random_uuid(), profiles.id, 'recruitment_post', sqlc.arg('post_id')::uuid
FROM profiles
WHERE profiles.id <> sqlc.arg('author_id')
  AND NOT EXISTS (
    SELECT 1 FROM notification_settings ns
    WHERE ns.profile_id = profiles.id AND ns.category = 'recruitment' AND NOT ns.in_app
  );

-- name: InsertMessageNotification :exec
-- 投稿や依頼にひもづかない知らせ。同じ場所への同じ見出しの未読があれば、新しく足さずにそれを最新にする。
WITH bumped AS (
    UPDATE notifications
    SET body = sqlc.arg('body'), created_at = now()
    WHERE recipient_id = sqlc.arg('recipient_id')
      AND kind = 'message'
      AND title = sqlc.arg('title')
      AND url = sqlc.arg('url')
      AND read_at IS NULL
    RETURNING id
)
INSERT INTO notifications (id, recipient_id, kind, title, body, url)
SELECT sqlc.arg('id'), sqlc.arg('recipient_id'), 'message', sqlc.arg('title'), sqlc.arg('body'), sqlc.arg('url')
WHERE NOT EXISTS (SELECT 1 FROM bumped);

-- name: InsertMonthlyReminderNotifications :exec
-- 月1回の記録のお知らせを、ベルで受け取る人全員に入れる。
INSERT INTO notifications (id, recipient_id, kind, title, body, url)
SELECT gen_random_uuid(), profiles.id, 'message', sqlc.arg('title'), sqlc.arg('body'), sqlc.arg('url')
FROM profiles
WHERE NOT EXISTS (
    SELECT 1 FROM notification_settings ns
    WHERE ns.profile_id = profiles.id AND ns.category = 'monthly' AND NOT ns.in_app
);

-- name: ListNotificationSettings :many
SELECT category, in_app, push FROM notification_settings
WHERE profile_id = $1;

-- name: GetNotificationSetting :one
SELECT in_app, push FROM notification_settings
WHERE profile_id = $1 AND category = $2;

-- name: UpsertNotificationSetting :exec
INSERT INTO notification_settings (profile_id, category, in_app, push)
VALUES ($1, $2, $3, $4)
ON CONFLICT (profile_id, category) DO UPDATE
SET in_app = EXCLUDED.in_app,
    push = EXCLUDED.push,
    updated_at = now();

-- name: ListPushRecipients :many
-- 通知をオンにした端末があり、その種類のプッシュを受け取る人。設定の行がない人は push_default に従う。
SELECT DISTINCT ps.profile_id
FROM push_subscriptions ps
LEFT JOIN notification_settings ns
  ON ns.profile_id = ps.profile_id AND ns.category = sqlc.arg('category')
WHERE ps.profile_id <> ALL (sqlc.arg('exclude')::uuid[])
  AND COALESCE(ns.push, sqlc.arg('push_default')::boolean)
ORDER BY ps.profile_id;

-- name: ListMyNotifications :many
SELECT
    n.id,
    n.kind,
    n.post_id,
    n.review_request_id,
    n.title,
    n.body,
    n.url,
    n.read_at,
    n.created_at,
    p.title AS post_title
FROM notifications n
LEFT JOIN posts p ON p.id = n.post_id
LEFT JOIN comments c ON c.id = n.comment_id
WHERE n.recipient_id = sqlc.arg('recipient_id')
  AND p.deleted_at IS NULL
  AND p.hidden_at IS NULL
  AND (
    n.comment_id IS NULL
    OR (c.deleted_at IS NULL AND c.hidden_at IS NULL)
  )
ORDER BY n.created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountMyNotifications :one
SELECT count(*)
FROM notifications n
LEFT JOIN posts p ON p.id = n.post_id
LEFT JOIN comments c ON c.id = n.comment_id
WHERE n.recipient_id = sqlc.arg('recipient_id')
  AND p.deleted_at IS NULL
  AND p.hidden_at IS NULL
  AND (
    n.comment_id IS NULL
    OR (c.deleted_at IS NULL AND c.hidden_at IS NULL)
  );

-- name: CountUnreadNotifications :one
SELECT count(*)
FROM notifications n
LEFT JOIN posts p ON p.id = n.post_id
LEFT JOIN comments c ON c.id = n.comment_id
WHERE n.recipient_id = sqlc.arg('recipient_id')
  AND n.read_at IS NULL
  AND p.deleted_at IS NULL
  AND p.hidden_at IS NULL
  AND (
    n.comment_id IS NULL
    OR (c.deleted_at IS NULL AND c.hidden_at IS NULL)
  );

-- name: MarkNotificationRead :execrows
UPDATE notifications
SET read_at = COALESCE(read_at, now())
WHERE id = sqlc.arg('id')
  AND recipient_id = sqlc.arg('recipient_id');

-- name: MarkAllNotificationsRead :exec
UPDATE notifications
SET read_at = now()
WHERE recipient_id = $1
  AND read_at IS NULL;
