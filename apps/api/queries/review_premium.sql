-- プレミアム（本体 #1053、補足 7231984993 の再設計）の動画グループ・動画と返却。

-- name: CreateReviewRequestVideoGroup :exec
INSERT INTO review_request_video_groups (request_id, position, kind, title, focus_note)
VALUES ($1, $2, $3, $4, $5);

-- name: ListReviewRequestVideoGroups :many
SELECT * FROM review_request_video_groups
WHERE request_id = $1
ORDER BY position;

-- name: CreateReviewRequestVideo :one
INSERT INTO review_request_videos (request_id, position, group_position, video_public_id, race_time, wind_speed, body_condition)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListReviewRequestVideos :many
SELECT * FROM review_request_videos
WHERE request_id = $1
ORDER BY position;

-- name: CreatePremiumReviewResponse :one
-- 最後の「まとめ」（summary）と任意の見本動画。スタンダードの型の欄と一言診断は NULL のまま。
INSERT INTO review_responses (request_id, coach_id, summary, demo_video_public_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: CreateReviewResponseSection :exec
INSERT INTO review_response_sections (request_id, group_position, body)
VALUES ($1, $2, $3);

-- name: ListReviewResponseSections :many
-- 並びはグループの順（選手が作った順）。
SELECT * FROM review_response_sections
WHERE request_id = $1
ORDER BY group_position;
