-- name: CreateSenseNote :one
INSERT INTO sense_notes (id, author_id, body, event)
VALUES ($1, $2, $3, sqlc.narg('event'))
RETURNING id, body, event, created_at, updated_at;

-- name: ListSenseNotes :many
-- 新しい順。cursor の時刻と id より古いものを返す（最初のページは cursor を渡さない）。
SELECT id, body, event, created_at, updated_at
FROM sense_notes
WHERE author_id = sqlc.arg('author_id')
  AND (
      sqlc.narg('before_at')::timestamptz IS NULL
      OR (created_at, id) < (sqlc.narg('before_at')::timestamptz, sqlc.narg('before_id')::uuid)
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('page_size');

-- name: UpdateSenseNote :one
UPDATE sense_notes
SET body = sqlc.arg('body'),
    event = sqlc.narg('event'),
    updated_at = now()
WHERE id = sqlc.arg('id')
  AND author_id = sqlc.arg('author_id')
RETURNING id, body, event, created_at, updated_at;

-- name: DeleteSenseNote :execrows
DELETE FROM sense_notes
WHERE id = $1
  AND author_id = $2;

-- name: CountSenseNotesSince :one
SELECT count(*) FROM sense_notes
WHERE author_id = $1
  AND created_at >= sqlc.arg('since');

-- name: CountSenseNotes :one
SELECT count(*) FROM sense_notes WHERE author_id = $1;

-- name: ListSenseNoteDays :many
-- 書いた日（日本時間）を新しい順に。連続日数と、書いた日数の計算に使う。
SELECT DISTINCT (created_at AT TIME ZONE 'Asia/Tokyo')::date AS day
FROM sense_notes
WHERE author_id = $1
ORDER BY day DESC;
