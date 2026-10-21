-- name: CreateSenseNote :one
INSERT INTO sense_notes (id, author_id, body, event, day)
VALUES ($1, $2, $3, sqlc.narg('event'), sqlc.arg('day'))
RETURNING id, body, event, day, created_at, updated_at;

-- name: ListSenseNotes :many
-- 書いた日の新しい順。cursor の日・時刻・id より後ろのものを返す（最初のページは cursor を渡さない）。
SELECT id, body, event, day, created_at, updated_at
FROM sense_notes
WHERE author_id = sqlc.arg('author_id')
  AND (
      sqlc.narg('before_day')::date IS NULL
      OR (day, created_at, id) < (sqlc.narg('before_day')::date, sqlc.narg('before_at')::timestamptz, sqlc.narg('before_id')::uuid)
  )
ORDER BY day DESC, created_at DESC, id DESC
LIMIT sqlc.arg('page_size');

-- name: UpdateSenseNote :one
UPDATE sense_notes
SET body = sqlc.arg('body'),
    event = sqlc.narg('event'),
    updated_at = now()
WHERE id = sqlc.arg('id')
  AND author_id = sqlc.arg('author_id')
RETURNING id, body, event, day, created_at, updated_at;

-- name: DeleteSenseNote :execrows
DELETE FROM sense_notes
WHERE id = $1
  AND author_id = $2;

-- name: LockSenseNoteAuthor :exec
-- 同じ人の感覚メモの作成とまとめを1つずつにする。1日1件の確認と作成のあいだに、別の作成を割り込ませない。
SELECT pg_advisory_xact_lock(hashtext('sense_notes:' || sqlc.arg(author_id)::uuid::text));

-- name: CountSenseNotesOnDay :one
SELECT count(*) FROM sense_notes
WHERE author_id = $1
  AND day = sqlc.arg('day');

-- name: CountSenseNotesSince :one
-- まとめにまだ使っていない感覚メモの数。since は前のまとめを作った時刻（まだ無ければ十分昔）。
SELECT count(*) FROM sense_notes
WHERE author_id = $1
  AND created_at > sqlc.arg('since');

-- name: CountSenseNotes :one
SELECT count(*) FROM sense_notes WHERE author_id = $1;

-- name: ListSenseNoteDays :many
-- 書いた日（日本時間）を新しい順に。連続日数と、書いた日数の計算に使う。
SELECT DISTINCT day
FROM sense_notes
WHERE author_id = $1
ORDER BY day DESC;

-- name: ListSenseNotesBetween :many
-- まとめに渡す感覚メモ（since より後に保存したもの）。多いときは新しい方から max_notes 件を、古い順に。
SELECT body, event, day, created_at
FROM (
    SELECT body, event, day, created_at, id
    FROM sense_notes
    WHERE author_id = sqlc.arg('author_id')
      AND created_at > sqlc.arg('since')
    ORDER BY created_at DESC, id DESC
    LIMIT sqlc.arg('max_notes')
) AS recent
ORDER BY day, created_at, id;

-- name: CreateSenseNoteSummary :one
INSERT INTO sense_note_summaries (id, author_id, from_day, to_day, note_count, summary)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, from_day, to_day, note_count, summary, created_at;

-- name: CountSenseNoteSummariesSince :one
-- その日(since は日本時間の0時)に作ったまとめの数。1日の上限に使う。
SELECT count(*) FROM sense_note_summaries
WHERE author_id = $1
  AND created_at >= sqlc.arg('since');

-- name: LatestSenseNoteSummary :one
SELECT id, from_day, to_day, note_count, summary, created_at
FROM sense_note_summaries
WHERE author_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: ClaimSenseNoteSummary :one
-- まとめを作り始める印を取る。ほかのリクエストが作っている最中(2分以内)なら行は返らない。
INSERT INTO sense_note_summary_claims (author_id, claimed_at)
VALUES ($1, now())
ON CONFLICT (author_id) DO UPDATE SET claimed_at = now()
WHERE sense_note_summary_claims.claimed_at < now() - interval '2 minutes'
RETURNING author_id;

-- name: ReleaseSenseNoteSummaryClaim :exec
DELETE FROM sense_note_summary_claims WHERE author_id = $1;
