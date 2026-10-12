-- name: UpsertRecruitmentDetail :exec
INSERT INTO recruitment_details (
    post_id, venue, schedule, gender_ratio, average_age, recruit_age
) VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (post_id) DO UPDATE SET
    venue = EXCLUDED.venue,
    schedule = EXCLUDED.schedule,
    gender_ratio = EXCLUDED.gender_ratio,
    average_age = EXCLUDED.average_age,
    recruit_age = EXCLUDED.recruit_age;

-- name: GetRecruitmentDetail :one
SELECT post_id, venue, schedule, gender_ratio, average_age, recruit_age
FROM recruitment_details
WHERE post_id = $1;

-- name: DeleteRecruitmentPrefectures :exec
DELETE FROM recruitment_prefectures WHERE post_id = $1;

-- name: InsertRecruitmentPrefecture :exec
INSERT INTO recruitment_prefectures (post_id, position, prefecture)
VALUES ($1, $2, $3);

-- name: ListRecruitmentPrefectures :many
SELECT prefecture
FROM recruitment_prefectures
WHERE post_id = $1
ORDER BY position;

-- name: DeleteRecruitmentLinks :exec
DELETE FROM recruitment_links WHERE post_id = $1;

-- name: InsertRecruitmentLink :exec
INSERT INTO recruitment_links (post_id, position, url)
VALUES ($1, $2, $3);

-- name: ListRecruitmentLinks :many
SELECT url
FROM recruitment_links
WHERE post_id = $1
ORDER BY position;

-- name: DeleteRecruitmentPhotos :exec
DELETE FROM recruitment_photos WHERE post_id = $1;

-- name: InsertRecruitmentPhoto :exec
INSERT INTO recruitment_photos (post_id, position, cloudinary_public_id)
VALUES ($1, $2, $3);

-- name: ListRecruitmentPhotos :many
SELECT position, cloudinary_public_id
FROM recruitment_photos
WHERE post_id = $1
ORDER BY position;

-- name: ListRecruitmentCovers :many
SELECT post_id, cloudinary_public_id
FROM recruitment_photos
WHERE position = 0
  AND post_id = ANY(sqlc.arg('post_ids')::uuid[]);
