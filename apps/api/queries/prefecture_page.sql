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
