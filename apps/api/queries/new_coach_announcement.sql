-- name: ClaimNewCoachAnnouncement :one
-- 新しいコーチの初回候補入りの発火を1回だけ記録する。既に記録があれば何も返さない（二重発火しない）。
-- 送信の記録（announcements 行）は配信の確定後に MarkNewCoachNotified で付ける。
INSERT INTO new_coach_announcements (coach_id)
VALUES ($1)
ON CONFLICT (coach_id) DO NOTHING
RETURNING *;

-- name: GetNewCoachAnnouncement :one
SELECT * FROM new_coach_announcements
WHERE coach_id = $1;

-- name: MarkNewCoachNotified :exec
UPDATE new_coach_announcements
SET notified = true, announcement_id = $2
WHERE coach_id = $1;

-- name: GetNewCoachDisplay :one
-- 自動通知の文面と宛先判定に使う、そのコーチの公開表示名・職員判定・状態。
SELECT COALESCE(NULLIF(c.name, ''), p.nickname, 'コーチ')::text AS display_name,
    p.is_admin AS is_admin, c.status AS status, c.on_duty AS on_duty, c.events AS events
FROM coach_profiles c JOIN profiles p ON p.id = c.profile_id
WHERE c.profile_id = $1;

-- name: InsertAnnouncementNotificationsExcluding :execrows
-- 運営からのお知らせを、指定の1人を除くベルで受け取る人全員に入れる。
-- 同じお知らせのベルがすでに入っていれば足さない（送り直しの再開で二重に入れない）。
INSERT INTO notifications (id, recipient_id, kind, title, body, url, announcement_id)
SELECT gen_random_uuid(), profiles.id, 'message', a.title, a.body, a.url, a.id
FROM profiles, announcements a
WHERE a.id = sqlc.arg('announcement_id')
  AND profiles.id <> sqlc.arg('exclude')::uuid
  AND NOT EXISTS (
    SELECT 1 FROM notification_settings ns
    WHERE ns.profile_id = profiles.id AND ns.category = 'announcement' AND NOT ns.in_app
  )
  AND NOT EXISTS (
    SELECT 1 FROM notifications n
    WHERE n.announcement_id = a.id
      AND n.recipient_id = profiles.id
  );
