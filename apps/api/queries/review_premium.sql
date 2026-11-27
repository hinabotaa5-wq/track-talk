-- プレミアム（本体 #1053）の動画と総括。

-- name: CreateReviewRequestVideo :one
INSERT INTO review_request_videos (request_id, position, slot, video_public_id, race_time, wind_speed, body_condition, practice_purpose)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListReviewRequestVideos :many
SELECT * FROM review_request_videos
WHERE request_id = $1
ORDER BY position;

-- name: CreatePremiumReviewResponse :one
-- 総括の全体の一言診断（hitokoto）と任意の見本動画。スタンダードの型の欄は NULL のまま。
INSERT INTO review_responses (request_id, coach_id, hitokoto, demo_video_public_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: CreateReviewResponseSection :exec
INSERT INTO review_response_sections (request_id, slot, body)
VALUES ($1, $2, $3);

-- name: ListReviewResponseSections :many
-- 並びは枠の決まった順（レース全体→スタート→後半→練習→その他）。
SELECT * FROM review_response_sections
WHERE request_id = $1
ORDER BY array_position(ARRAY['race', 'start', 'second_half', 'practice', 'other']::text[], slot);
