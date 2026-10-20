-- name: CreateRaceRecord :one
INSERT INTO race_records (id, author_id, raced_on, event, mark, wind, memo)
VALUES (sqlc.arg('id'), sqlc.arg('author_id'), sqlc.arg('raced_on'), sqlc.arg('event'), sqlc.arg('mark'), sqlc.narg('wind'), sqlc.arg('memo'))
RETURNING id, raced_on, event, mark, wind, memo, created_at, updated_at;

-- name: ListRaceRecords :many
-- 古い順。自己ベストの判定を、日付の順にたどって行うため。
SELECT id, raced_on, event, mark, wind, memo, created_at, updated_at
FROM race_records
WHERE author_id = sqlc.arg('author_id')
ORDER BY event, raced_on, created_at, id;

-- name: LockRaceRecordAuthor :exec
-- 同じ人の記録の作成を1つずつにする。1000件の上限の確認と作成のあいだに、別の作成を割り込ませない。
SELECT pg_advisory_xact_lock(hashtext('race_records:' || sqlc.arg(author_id)::uuid::text));

-- name: CountRaceRecords :one
SELECT count(*) FROM race_records WHERE author_id = $1;

-- name: UpdateRaceRecord :one
UPDATE race_records
SET raced_on = sqlc.arg('raced_on'),
    event = sqlc.arg('event'),
    mark = sqlc.arg('mark'),
    wind = sqlc.narg('wind'),
    memo = sqlc.arg('memo'),
    updated_at = now()
WHERE id = sqlc.arg('id')
  AND author_id = sqlc.arg('author_id')
RETURNING id, raced_on, event, mark, wind, memo, created_at, updated_at;

-- name: DeleteRaceRecord :execrows
DELETE FROM race_records
WHERE id = $1
  AND author_id = $2;
