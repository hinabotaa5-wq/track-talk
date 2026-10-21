-- name: CreateRaceRecord :one
INSERT INTO race_records (id, author_id, raced_on, event, mark, wind, memo, meet_name)
VALUES (sqlc.arg('id'), sqlc.arg('author_id'), sqlc.arg('raced_on'), sqlc.arg('event'), sqlc.arg('mark'), sqlc.narg('wind'), sqlc.arg('memo'), sqlc.arg('meet_name'))
RETURNING id, raced_on, event, mark, wind, memo, meet_name, created_at, updated_at;

-- name: ListRaceRecords :many
-- 古い順。自己ベストの判定を、日付の順にたどって行うため。
SELECT id, raced_on, event, mark, wind, memo, meet_name, created_at, updated_at
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
    meet_name = sqlc.arg('meet_name'),
    updated_at = now()
WHERE id = sqlc.arg('id')
  AND author_id = sqlc.arg('author_id')
RETURNING id, raced_on, event, mark, wind, memo, meet_name, created_at, updated_at;

-- name: DeleteRaceRecord :execrows
DELETE FROM race_records
WHERE id = $1
  AND author_id = $2;

-- name: CountRecentPBAuthors :many
-- 種目ごとに、期間内の日付で自己ベストを更新した人の数。自分の前の公認記録を上回った記録だけを数え、
-- その種目の最初の記録は数えない。追い風参考(+2.0m/s 超)は比べる対象にも入れない。
-- 記録が大きいほどよい種目は higher_events で受け取り、符号を反転して「小さいほどよい」にそろえる。
WITH legal AS (
    SELECT
        author_id,
        event,
        raced_on,
        CASE WHEN event = ANY(sqlc.arg(higher_events)::text[]) THEN -mark ELSE mark END AS score,
        min(CASE WHEN event = ANY(sqlc.arg(higher_events)::text[]) THEN -mark ELSE mark END) OVER (
            PARTITION BY author_id, event
            ORDER BY raced_on, created_at, id
            ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING
        ) AS prev_best
    FROM race_records
    WHERE wind IS NULL OR wind <= 20
)
SELECT event, count(DISTINCT author_id)::bigint AS authors
FROM legal
WHERE prev_best IS NOT NULL
  AND score < prev_best
  AND raced_on >= sqlc.arg(since)::date
  AND raced_on <= sqlc.arg(until)::date
GROUP BY event
ORDER BY authors DESC, event;
