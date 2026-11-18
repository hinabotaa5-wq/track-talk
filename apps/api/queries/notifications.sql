-- name: InsertNotification :exec
INSERT INTO notifications (id, recipient_id, kind, post_id, comment_id, application_id)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: InsertReviewNotification :exec
INSERT INTO notifications (id, recipient_id, kind, review_request_id, url, coaching_role)
VALUES ($1, $2, $3, $4, $5, CASE
    WHEN $3 IN ('review_offer', 'review_offer_reminder', 'review_resubmitted',
        'review_resubmitted_after_accept', 'review_nomination_ended', 'review_clarify') THEN 'coach'
    ELSE 'player' END);

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
    SET body = sqlc.arg('body'), created_at = now(), coaching_role = sqlc.narg('coaching_role')::text
    WHERE recipient_id = sqlc.arg('recipient_id')
      AND kind = 'message'
      AND announcement_id IS NULL
      AND title = sqlc.arg('title')
      AND url = sqlc.arg('url')
      AND read_at IS NULL
    RETURNING id
)
INSERT INTO notifications (id, recipient_id, kind, title, body, url, coaching_role)
SELECT sqlc.arg('id'), sqlc.arg('recipient_id'), 'message', sqlc.arg('title'), sqlc.arg('body'), sqlc.arg('url'), sqlc.narg('coaching_role')::text
WHERE NOT EXISTS (SELECT 1 FROM bumped);

-- name: InsertMonthlyReminderNotifications :exec
-- 月1回の記録のお知らせを、ベルで受け取る人全員に入れる。文面は本人の表示言語に合わせる。
INSERT INTO notifications (id, recipient_id, kind, title, body, url)
SELECT gen_random_uuid(), profiles.id, 'message',
    CASE WHEN profiles.locale = 'en' THEN sqlc.arg('title_en') ELSE sqlc.arg('title') END AS title,
    CASE WHEN profiles.locale = 'en' THEN sqlc.arg('body_en') ELSE sqlc.arg('body') END AS body,
    sqlc.arg('url') AS url
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
  AND n.coaching_role IS NULL
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
  AND n.coaching_role IS NULL
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
  AND n.coaching_role IS NULL
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
  AND coaching_role IS NULL
  AND read_at IS NULL;

-- name: CoachingContractNotificationRole :one
SELECT CASE WHEN coach_id = sqlc.arg('recipient_id')::uuid THEN 'coach'
            WHEN requester_id = sqlc.arg('recipient_id')::uuid THEN 'player' ELSE '' END::text AS role
FROM coaching_contracts WHERE id = sqlc.arg('contract_id')::uuid;

-- name: MarkCoachingNotificationsRead :exec
-- Only the authenticated recipient's matching role and screen; ordinary bell rows are untouched.
UPDATE notifications SET read_at = now()
WHERE recipient_id = sqlc.arg('recipient_id')::uuid
  AND coaching_role = sqlc.arg('coaching_role')::text
  AND read_at IS NULL
  AND (split_part(url, '#', 1) = ANY(sqlc.arg('urls')::text[])
       OR (url = '/me' AND title LIKE '紹介特典：%' AND '/me/coaching/tickets' = ANY(sqlc.arg('urls')::text[]))
       OR (sqlc.narg('review_request_id')::uuid IS NOT NULL
           AND review_request_id = sqlc.narg('review_request_id')::uuid));

-- name: DeleteOldNotificationsFor :exec
-- その人の古い通知を消す。読んだものは届いてから90日、読んでいないものは180日で消す(プライバシーポリシー第10条)。
DELETE FROM notifications
WHERE recipient_id = $1
  AND (
      (read_at IS NOT NULL AND created_at < now() - interval '90 days')
      OR created_at < now() - interval '180 days'
  );

-- name: DeleteOldNotifications :execrows
-- 全員の古い通知を消す。月1回のお知らせのときに流し、ベルを開かない人の通知もたまらないようにする。
DELETE FROM notifications
WHERE (read_at IS NOT NULL AND created_at < now() - interval '90 days')
   OR created_at < now() - interval '180 days';
