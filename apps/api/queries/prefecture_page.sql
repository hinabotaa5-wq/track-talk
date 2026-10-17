-- name: ListUpcomingPrefectureCompetitions :many
-- 県ページに出す、これからの大会(開催中を含む)。日付の近い順。
SELECT id, name, starts_on, ends_on, location
FROM competitions
WHERE prefecture = sqlc.arg(prefecture)
  AND COALESCE(ends_on, starts_on) >= sqlc.arg(today)::date
ORDER BY starts_on ASC, name ASC
LIMIT sqlc.arg(row_limit);

-- name: CountUpcomingPrefectureCompetitions :one
SELECT count(*)
FROM competitions
WHERE prefecture = sqlc.arg(prefecture)
  AND COALESCE(ends_on, starts_on) >= sqlc.arg(today)::date;

-- name: ListPrefectureRecruitments :many
-- 県ページに出す、クラブチームの募集。新しい順。
SELECT p.id, p.title, p.created_at, d.venue, d.schedule
FROM posts p
JOIN recruitment_prefectures rp ON rp.post_id = p.id
JOIN recruitment_details d ON d.post_id = p.id
WHERE rp.prefecture = sqlc.arg(prefecture)
  AND p.kind = 'recruitment'
  AND p.deleted_at IS NULL
  AND p.hidden_at IS NULL
ORDER BY p.created_at DESC, p.id DESC
LIMIT sqlc.arg(row_limit);

-- name: CountPrefectureRecruitments :one
SELECT count(*)
FROM posts p
JOIN recruitment_prefectures rp ON rp.post_id = p.id
WHERE rp.prefecture = sqlc.arg(prefecture)
  AND p.kind = 'recruitment'
  AND p.deleted_at IS NULL
  AND p.hidden_at IS NULL;
