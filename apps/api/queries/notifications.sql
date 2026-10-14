-- name: InsertNotification :exec
INSERT INTO notifications (id, recipient_id, kind, post_id, comment_id, application_id)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListMyNotifications :many
SELECT
    n.id,
    n.kind,
    n.post_id,
    n.read_at,
    n.created_at,
    p.title AS post_title
FROM notifications n
JOIN posts p ON p.id = n.post_id
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
JOIN posts p ON p.id = n.post_id
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
JOIN posts p ON p.id = n.post_id
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
