-- name: CreateAnnouncement :one
-- 送信の記録を作る。同じ id がもう送られていたら、何も返さない(二重に送らない)。
INSERT INTO announcements (id, sender_id, title, body, url)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (id) DO NOTHING
RETURNING *;

-- name: RecentSameAnnouncement :one
-- 同じ見出しと本文を、この1時間に送っていないか。取り消したものは数えない。
SELECT count(*) FROM announcements
WHERE title = $1 AND body = $2
  AND canceled_at IS NULL
  AND created_at > now() - interval '1 hour';

-- name: InsertAnnouncementNotifications :execrows
-- 運営からのお知らせを、ベルで受け取る人全員に入れる。
-- 同じお知らせのベルがすでに入っていれば足さない（送り直しの再開で二重に入れない）。
INSERT INTO notifications (id, recipient_id, kind, title, body, url, announcement_id)
SELECT gen_random_uuid(), profiles.id, 'message', a.title, a.body, a.url, a.id
FROM profiles, announcements a
WHERE a.id = sqlc.arg('announcement_id')
  AND NOT EXISTS (
    SELECT 1 FROM notification_settings ns
    WHERE ns.profile_id = profiles.id AND ns.category = 'announcement' AND NOT ns.in_app
  )
  AND NOT EXISTS (
    SELECT 1 FROM notifications n
    WHERE n.announcement_id = a.id
  );

-- name: SetAnnouncementInAppCount :exec
UPDATE announcements SET in_app_count = $2 WHERE id = $1;

-- name: FinishAnnouncement :exec
UPDATE announcements SET finished_at = now(), push_recipients = $2 WHERE id = $1;

-- name: GetAnnouncement :one
-- 送り直し（再開）のとき、同じ id のお知らせの中身を確かめる。
SELECT id, sender_id, title, body, url, created_at, in_app_count, finished_at, push_recipients, canceled_at
FROM announcements
WHERE id = $1;

-- name: CountAnnouncementNotifications :one
-- 入れたベルの数。送り直しの再開で、すでに入っているか確かめる。
SELECT count(*) FROM notifications
WHERE announcement_id = $1;

-- name: CountAnnouncementRecipients :one
-- 送る前に確認画面へ出す人数。ベルはアプリ内で受け取る人、プッシュは通知をオンにした端末がある人。
SELECT
    (SELECT count(*) FROM profiles p
     WHERE NOT EXISTS (
         SELECT 1 FROM notification_settings ns
         WHERE ns.profile_id = p.id AND ns.category = 'announcement' AND NOT ns.in_app
     ))::bigint AS in_app,
    (SELECT count(DISTINCT ps.profile_id) FROM push_subscriptions ps
     LEFT JOIN notification_settings ns
       ON ns.profile_id = ps.profile_id AND ns.category = 'announcement'
     WHERE COALESCE(ns.push, sqlc.arg('push_default')::boolean))::bigint AS push;

-- name: ListAnnouncements :many
SELECT a.*, p.nickname AS sender_name
FROM announcements a
LEFT JOIN profiles p ON p.id = a.sender_id
ORDER BY a.created_at DESC
LIMIT 20;

-- name: CancelAnnouncement :one
UPDATE announcements SET canceled_at = now()
WHERE id = $1 AND canceled_at IS NULL
RETURNING id;

-- name: DeleteAnnouncementNotifications :execrows
DELETE FROM notifications WHERE announcement_id = $1;
