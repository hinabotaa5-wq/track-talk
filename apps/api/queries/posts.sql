-- name: ListPostsNew :many
SELECT
    p.id,
    p.author_id,
    p.title,
    p.body,
    p.kind,
    p.youtube_video_id,
    p.youtube_thumbnail_url,
    p.instagram_type,
    p.instagram_shortcode,
    p.cloudinary_public_id,
    p.source_title,
    p.like_count,
    p.comment_count,
    p.created_at,
    p.updated_at
FROM posts p
WHERE p.deleted_at IS NULL
  AND p.hidden_at IS NULL
  AND (sqlc.narg('author_id')::uuid IS NULL OR p.author_id = sqlc.narg('author_id'))
  AND (
    sqlc.narg('part')::text IS NULL
    OR EXISTS (
      SELECT 1 FROM injury_details i
      WHERE i.post_id = p.id
        AND (sqlc.narg('part')::text IS NULL OR i.body_part = sqlc.narg('part'))
    )
  )
  AND (
    sqlc.narg('stage')::text IS NULL
    OR COALESCE(
      (
        SELECT u.stage FROM injury_updates u
        WHERE u.post_id = p.id
        ORDER BY u.created_at DESC
        LIMIT 1
      ),
      (SELECT d.group_stage FROM injury_details d WHERE d.post_id = p.id)
    ) = sqlc.narg('stage')
  )
  AND (
    (sqlc.narg('kind')::text IS NULL AND (p.kind NOT IN ('injury', 'meet') OR sqlc.narg('author_id')::uuid IS NOT NULL))
    OR (
      sqlc.narg('kind') = 'recruitment'
      AND p.kind IN ('recruitment', 'practice')
    )
    OR (
      sqlc.narg('kind') <> 'recruitment'
      AND p.kind = sqlc.narg('kind')
    )
  )
  AND (
    sqlc.narg('keyword')::text IS NULL
    OR p.title ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
    OR p.body ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
    OR EXISTS (
      SELECT 1 FROM recruitment_details d
      WHERE d.post_id = p.id
        AND (
          d.venue ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
          OR d.schedule ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
          OR d.gender_ratio ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
          OR d.average_age ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
          OR d.recruit_age ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
        )
    )
    OR EXISTS (
      SELECT 1 FROM practice_details pd
      WHERE pd.post_id = p.id
        AND (
          pd.venue ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
          OR pd.event_other ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
        )
    )
    OR (
      cardinality(sqlc.arg('prefecture_codes')::text[]) > 0
      AND EXISTS (
        SELECT 1 FROM recruitment_prefectures rp
        WHERE rp.post_id = p.id
          AND rp.prefecture = ANY(sqlc.arg('prefecture_codes')::text[])
      )
    )
  )
ORDER BY p.created_at DESC
LIMIT sqlc.arg('limit_n') OFFSET sqlc.arg('offset_n');

-- name: ListPostsPopular :many
SELECT
    p.id,
    p.author_id,
    p.title,
    p.body,
    p.kind,
    p.youtube_video_id,
    p.youtube_thumbnail_url,
    p.instagram_type,
    p.instagram_shortcode,
    p.cloudinary_public_id,
    p.source_title,
    p.like_count,
    p.comment_count,
    p.created_at,
    p.updated_at
FROM posts p
WHERE p.deleted_at IS NULL
  AND p.hidden_at IS NULL
  AND (sqlc.narg('popular_window')::bool IS NOT TRUE OR p.created_at >= now() - interval '14 days')
  AND (sqlc.narg('author_id')::uuid IS NULL OR p.author_id = sqlc.narg('author_id'))
  AND (
    sqlc.narg('part')::text IS NULL
    OR EXISTS (
      SELECT 1 FROM injury_details i
      WHERE i.post_id = p.id
        AND (sqlc.narg('part')::text IS NULL OR i.body_part = sqlc.narg('part'))
    )
  )
  AND (
    sqlc.narg('stage')::text IS NULL
    OR COALESCE(
      (
        SELECT u.stage FROM injury_updates u
        WHERE u.post_id = p.id
        ORDER BY u.created_at DESC
        LIMIT 1
      ),
      (SELECT d.group_stage FROM injury_details d WHERE d.post_id = p.id)
    ) = sqlc.narg('stage')
  )
  AND (
    (sqlc.narg('kind')::text IS NULL AND (p.kind NOT IN ('injury', 'meet') OR sqlc.narg('author_id')::uuid IS NOT NULL))
    OR (
      sqlc.narg('kind') = 'recruitment'
      AND p.kind IN ('recruitment', 'practice')
    )
    OR (
      sqlc.narg('kind') <> 'recruitment'
      AND p.kind = sqlc.narg('kind')
    )
  )
  AND (
    sqlc.narg('keyword')::text IS NULL
    OR p.title ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
    OR p.body ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
    OR EXISTS (
      SELECT 1 FROM recruitment_details d
      WHERE d.post_id = p.id
        AND (
          d.venue ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
          OR d.schedule ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
          OR d.gender_ratio ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
          OR d.average_age ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
          OR d.recruit_age ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
        )
    )
    OR EXISTS (
      SELECT 1 FROM practice_details pd
      WHERE pd.post_id = p.id
        AND (
          pd.venue ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
          OR pd.event_other ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
        )
    )
    OR (
      cardinality(sqlc.arg('prefecture_codes')::text[]) > 0
      AND EXISTS (
        SELECT 1 FROM recruitment_prefectures rp
        WHERE rp.post_id = p.id
          AND rp.prefecture = ANY(sqlc.arg('prefecture_codes')::text[])
      )
    )
  )
ORDER BY p.like_count DESC, p.created_at DESC
LIMIT sqlc.arg('limit_n') OFFSET sqlc.arg('offset_n');

-- name: CountPosts :one
SELECT count(*)
FROM posts p
WHERE p.deleted_at IS NULL
  AND p.hidden_at IS NULL
  AND (sqlc.narg('author_id')::uuid IS NULL OR p.author_id = sqlc.narg('author_id'))
  AND (
    sqlc.narg('part')::text IS NULL
    OR EXISTS (
      SELECT 1 FROM injury_details i
      WHERE i.post_id = p.id
        AND (sqlc.narg('part')::text IS NULL OR i.body_part = sqlc.narg('part'))
    )
  )
  AND (
    sqlc.narg('stage')::text IS NULL
    OR COALESCE(
      (
        SELECT u.stage FROM injury_updates u
        WHERE u.post_id = p.id
        ORDER BY u.created_at DESC
        LIMIT 1
      ),
      (SELECT d.group_stage FROM injury_details d WHERE d.post_id = p.id)
    ) = sqlc.narg('stage')
  )
  AND (
    (sqlc.narg('kind')::text IS NULL AND (p.kind NOT IN ('injury', 'meet') OR sqlc.narg('author_id')::uuid IS NOT NULL))
    OR (
      sqlc.narg('kind') = 'recruitment'
      AND p.kind IN ('recruitment', 'practice')
    )
    OR (
      sqlc.narg('kind') <> 'recruitment'
      AND p.kind = sqlc.narg('kind')
    )
  )
  AND (sqlc.narg('popular_window')::bool IS NOT TRUE OR p.created_at >= now() - interval '14 days')
  AND (
    sqlc.narg('keyword')::text IS NULL
    OR p.title ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
    OR p.body ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
    OR EXISTS (
      SELECT 1 FROM recruitment_details d
      WHERE d.post_id = p.id
        AND (
          d.venue ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
          OR d.schedule ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
          OR d.gender_ratio ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
          OR d.average_age ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
          OR d.recruit_age ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
        )
    )
    OR EXISTS (
      SELECT 1 FROM practice_details pd
      WHERE pd.post_id = p.id
        AND (
          pd.venue ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
          OR pd.event_other ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
        )
    )
    OR (
      cardinality(sqlc.arg('prefecture_codes')::text[]) > 0
      AND EXISTS (
        SELECT 1 FROM recruitment_prefectures rp
        WHERE rp.post_id = p.id
          AND rp.prefecture = ANY(sqlc.arg('prefecture_codes')::text[])
      )
    )
  );

-- name: FindYouTubePostID :one
SELECT id
FROM posts
WHERE youtube_video_id = $1
  AND deleted_at IS NULL
  AND hidden_at IS NULL;

-- name: FindInstagramPostID :one
SELECT id
FROM posts
WHERE instagram_shortcode = $1
  AND deleted_at IS NULL
  AND hidden_at IS NULL;

-- name: GetPost :one
SELECT
    p.id,
    p.author_id,
    p.title,
    p.body,
    p.kind,
    p.youtube_video_id,
    p.youtube_thumbnail_url,
    p.instagram_type,
    p.instagram_shortcode,
    p.cloudinary_public_id,
    p.source_title,
    p.like_count,
    p.comment_count,
    profile.nickname AS author_name,
    -- 投稿マスターのバッジ用。ニックネームとバッジを公開している人だけ数える。
    -- 自分でつけたいいねは数えない(自分の投稿にいいねを押すだけで付いてしまうので)。
    CASE WHEN profile.id IS NULL OR NOT profile.post_master_badge_public THEN 0 ELSE (
        SELECT count(*) FROM posts liked
        WHERE liked.author_id = p.author_id
          AND EXISTS (SELECT 1 FROM post_likes l WHERE l.post_id = liked.id AND l.user_id <> liked.author_id)
          AND liked.deleted_at IS NULL
          AND liked.hidden_at IS NULL
    ) END::bigint AS author_liked_post_count,
    p.created_at,
    p.updated_at
FROM posts p
LEFT JOIN profiles profile
    ON profile.id = p.author_id
   AND profile.nickname_public
WHERE p.id = $1
  AND p.deleted_at IS NULL
  AND p.hidden_at IS NULL;

-- name: CreatePost :one
INSERT INTO posts (
    id, author_id, title, body,
    kind, youtube_video_id, youtube_thumbnail_url, cloudinary_public_id,
    instagram_type, instagram_shortcode, source_title
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING id, author_id, title, body, kind, youtube_video_id, youtube_thumbnail_url, cloudinary_public_id, instagram_type, instagram_shortcode, source_title, like_count, comment_count, created_at, updated_at;

-- name: UpdatePost :one
UPDATE posts
SET title = $3,
    body = $4,
    updated_at = now()
WHERE id = $1
  AND author_id = $2
  AND deleted_at IS NULL
  AND hidden_at IS NULL
RETURNING id, author_id, title, body, like_count, comment_count, created_at, updated_at;

-- name: GetPostMedia :one
-- 投稿を消す前に、使っている写真・動画を集めるための読み取り。非表示の投稿も読む。
SELECT id, kind, cloudinary_public_id FROM posts WHERE id = $1 AND deleted_at IS NULL;

-- name: SoftDeletePost :execrows
UPDATE posts
SET deleted_at = now(), updated_at = now()
WHERE id = $1
  AND author_id = $2
  AND deleted_at IS NULL;

-- name: HidePost :execrows
UPDATE posts
SET hidden_at = now(), updated_at = now()
WHERE id = $1
  AND deleted_at IS NULL
  AND hidden_at IS NULL;

-- name: ListSavedPosts :many
SELECT
    p.id,
    p.author_id,
    p.title,
    p.body,
    p.kind,
    p.youtube_video_id,
    p.youtube_thumbnail_url,
    p.instagram_type,
    p.instagram_shortcode,
    p.cloudinary_public_id,
    p.source_title,
    p.like_count,
    p.comment_count,
    p.created_at,
    p.updated_at
FROM post_saves s
JOIN posts p ON p.id = s.post_id
WHERE s.user_id = sqlc.arg('user_id')
  AND p.deleted_at IS NULL
  AND p.hidden_at IS NULL
  -- ケガの投稿は、いまそのグループに参加している間だけ出す(抜けたら読めない)。保存は残すので、入り直すとまた出る。
  AND (
    p.kind <> 'injury'
    OR p.author_id = sqlc.arg('user_id')
    OR EXISTS (
      SELECT 1 FROM injury_details i
      JOIN injury_group_members m ON m.body_part = i.body_part AND m.user_id = sqlc.arg('user_id')
      WHERE i.post_id = p.id
    )
  )
  AND (
    sqlc.narg('keyword')::text IS NULL
    OR p.title ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
    OR p.body ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
    OR EXISTS (
      SELECT 1 FROM recruitment_details d
      WHERE d.post_id = p.id
        AND (
          d.venue ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
          OR d.schedule ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
          OR d.gender_ratio ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
          OR d.average_age ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
          OR d.recruit_age ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
        )
    )
    OR EXISTS (
      SELECT 1 FROM practice_details pd
      WHERE pd.post_id = p.id
        AND (
          pd.venue ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
          OR pd.event_other ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
        )
    )
    OR (
      cardinality(sqlc.arg('prefecture_codes')::text[]) > 0
      AND EXISTS (
        SELECT 1 FROM recruitment_prefectures rp
        WHERE rp.post_id = p.id
          AND rp.prefecture = ANY(sqlc.arg('prefecture_codes')::text[])
      )
    )
  )
ORDER BY s.created_at DESC, p.id DESC
LIMIT sqlc.arg('limit_n') OFFSET sqlc.arg('offset_n');

-- name: CountSavedPosts :one
SELECT count(*)
FROM post_saves s
JOIN posts p ON p.id = s.post_id
WHERE s.user_id = sqlc.arg('user_id')
  AND p.deleted_at IS NULL
  AND p.hidden_at IS NULL
  -- ケガの投稿は、いまそのグループに参加している間だけ出す(抜けたら読めない)。保存は残すので、入り直すとまた出る。
  AND (
    p.kind <> 'injury'
    OR p.author_id = sqlc.arg('user_id')
    OR EXISTS (
      SELECT 1 FROM injury_details i
      JOIN injury_group_members m ON m.body_part = i.body_part AND m.user_id = sqlc.arg('user_id')
      WHERE i.post_id = p.id
    )
  )
  AND (
    sqlc.narg('keyword')::text IS NULL
    OR p.title ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
    OR p.body ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
    OR EXISTS (
      SELECT 1 FROM recruitment_details d
      WHERE d.post_id = p.id
        AND (
          d.venue ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
          OR d.schedule ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
          OR d.gender_ratio ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
          OR d.average_age ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
          OR d.recruit_age ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
        )
    )
    OR EXISTS (
      SELECT 1 FROM practice_details pd
      WHERE pd.post_id = p.id
        AND (
          pd.venue ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
          OR pd.event_other ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
        )
    )
    OR (
      cardinality(sqlc.arg('prefecture_codes')::text[]) > 0
      AND EXISTS (
        SELECT 1 FROM recruitment_prefectures rp
        WHERE rp.post_id = p.id
          AND rp.prefecture = ANY(sqlc.arg('prefecture_codes')::text[])
      )
    )
  );

-- name: SavedByUser :one
SELECT EXISTS(
    SELECT 1 FROM post_saves WHERE post_id = $1 AND user_id = $2
);

-- name: ListSavedPostIDs :many
SELECT post_id
FROM post_saves
WHERE user_id = $1
  AND post_id = ANY(sqlc.arg('post_ids')::uuid[]);

-- name: InsertSave :execrows
INSERT INTO post_saves (post_id, user_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: DeleteSave :execrows
DELETE FROM post_saves
WHERE post_id = $1 AND user_id = $2;

-- name: LikedByUser :one
SELECT EXISTS(
    SELECT 1 FROM post_likes WHERE post_id = $1 AND user_id = $2
);

-- name: ListLikedPostIDs :many
SELECT post_id
FROM post_likes
WHERE user_id = $1
  AND post_id = ANY(sqlc.arg('post_ids')::uuid[]);

-- name: InsertLike :execrows
INSERT INTO post_likes (post_id, user_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: DeleteLike :execrows
DELETE FROM post_likes
WHERE post_id = $1 AND user_id = $2;

-- name: IncrementLikeCount :exec
UPDATE posts SET like_count = like_count + 1 WHERE id = $1;

-- name: DecrementLikeCount :exec
UPDATE posts SET like_count = GREATEST(like_count - 1, 0) WHERE id = $1;

-- name: IncrementCommentCount :exec
UPDATE posts SET comment_count = comment_count + 1 WHERE id = $1;

-- name: DecrementCommentCount :exec
UPDATE posts SET comment_count = GREATEST(comment_count - 1, 0) WHERE id = $1;

-- name: ListMediaInUse :many
SELECT p.cloudinary_public_id::text AS public_id
FROM posts p
WHERE p.deleted_at IS NULL
  AND p.cloudinary_public_id = ANY(sqlc.arg('public_ids')::text[])
UNION
SELECT rp.cloudinary_public_id AS public_id
FROM recruitment_photos rp
JOIN posts p ON p.id = rp.post_id
WHERE p.deleted_at IS NULL
  AND rp.cloudinary_public_id = ANY(sqlc.arg('public_ids')::text[])
UNION
-- コーチの顔写真も同じフォルダにあり、一覧で公開 ID が見えるので、使っているものとして数える。
SELECT cp.photo_public_id::text AS public_id
FROM coach_profiles cp
WHERE cp.photo_public_id = ANY(sqlc.arg('public_ids')::text[]);
