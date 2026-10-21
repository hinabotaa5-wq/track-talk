-- name: UpsertMenuContestEntry :one
-- 1人1回1件。応募期間中は書き直せる。
INSERT INTO menu_contest_entries (id, round_start, author_id, body)
VALUES ($1, $2, $3, $4)
ON CONFLICT (round_start, author_id) DO UPDATE
SET body = EXCLUDED.body,
    updated_at = now()
RETURNING id, body, created_at, updated_at;

-- name: GetMyMenuContestEntry :one
SELECT id, body, created_at, updated_at
FROM menu_contest_entries
WHERE round_start = $1
  AND author_id = $2;

-- name: DeleteMyMenuContestEntry :execrows
DELETE FROM menu_contest_entries
WHERE round_start = $1
  AND author_id = $2;

-- name: ListMenuContestEntries :many
-- 運営が読む一覧。古い順。ニックネームは公開設定にかかわらず運営には見せる。
SELECT
    e.id,
    e.body,
    e.created_at,
    e.updated_at,
    p.nickname AS author_name,
    (f.entry_id IS NOT NULL)::boolean AS finalist
FROM menu_contest_entries e
JOIN profiles p ON p.id = e.author_id
LEFT JOIN menu_contest_finalists f ON f.entry_id = e.id
WHERE e.round_start = $1
ORDER BY e.created_at, e.id;

-- name: ListMenuContestFinalists :many
-- トップ3と得票数。ニックネームは本人が公開にしているときだけ見せる(Go 側で判断する)。
SELECT
    e.id,
    e.author_id,
    e.body,
    p.nickname AS author_nickname,
    p.nickname_public AS author_nickname_public,
    (SELECT count(*) FROM menu_contest_votes v WHERE v.entry_id = e.id)::bigint AS votes
FROM menu_contest_finalists f
JOIN menu_contest_entries e ON e.id = f.entry_id
JOIN profiles p ON p.id = e.author_id
WHERE f.round_start = $1
ORDER BY e.created_at, e.id;

-- name: DeleteMenuContestFinalists :exec
DELETE FROM menu_contest_finalists WHERE round_start = $1;

-- name: InsertMenuContestFinalist :execrows
-- その回の応募だけを選べる。
INSERT INTO menu_contest_finalists (round_start, entry_id)
SELECT e.round_start, e.id
FROM menu_contest_entries e
WHERE e.id = sqlc.arg('entry_id')
  AND e.round_start = sqlc.arg('round_start');

-- name: LockMenuContestRound :exec
-- トップ3の選び直しと投票を1つずつにする。投票が0件の確認から選び直しまでのあいだに票を割り込ませない。
SELECT pg_advisory_xact_lock(hashtext('menu_contest:' || sqlc.arg(round_start)::date::text));

-- name: CountMenuContestVotes :one
SELECT count(*) FROM menu_contest_votes WHERE round_start = $1;

-- name: UpsertMenuContestVote :exec
INSERT INTO menu_contest_votes (round_start, voter_id, entry_id)
VALUES ($1, $2, $3)
ON CONFLICT (round_start, voter_id) DO UPDATE
SET entry_id = EXCLUDED.entry_id,
    updated_at = now();

-- name: GetMyMenuContestVote :one
SELECT entry_id FROM menu_contest_votes
WHERE round_start = $1
  AND voter_id = $2;

-- name: LatestMenuContestResultRound :one
-- 投票が締め切られた回のうち、トップ3が選ばれた一番新しい回。
SELECT round_start
FROM menu_contest_finalists
WHERE round_start <= sqlc.arg('cutoff')
ORDER BY round_start DESC
LIMIT 1;
