-- name: ListPostsNew :many
SELECT
    p.id,
    p.author_id,
    p.title,
    p.body,
    p.kind,
    p.youtube_video_id,
    p.youtube_thumbnail_url,
    p.cloudinary_public_id,
    p.like_count,
    p.comment_count,
    p.created_at,
    p.updated_at
FROM posts p
WHERE p.deleted_at IS NULL
  AND p.hidden_at IS NULL
  AND (sqlc.narg('author_id')::uuid IS NULL OR p.author_id = sqlc.narg('author_id'))
  AND (
    sqlc.narg('keyword')::text IS NULL
    OR p.title ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
    OR p.body ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
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
    p.cloudinary_public_id,
    p.like_count,
    p.comment_count,
    p.created_at,
    p.updated_at
FROM posts p
WHERE p.deleted_at IS NULL
  AND p.hidden_at IS NULL
  AND p.created_at >= now() - interval '14 days'
  AND (sqlc.narg('author_id')::uuid IS NULL OR p.author_id = sqlc.narg('author_id'))
  AND (
    sqlc.narg('keyword')::text IS NULL
    OR p.title ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
    OR p.body ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
  )
ORDER BY p.like_count DESC, p.created_at DESC
LIMIT sqlc.arg('limit_n') OFFSET sqlc.arg('offset_n');

-- name: CountPosts :one
SELECT count(*)
FROM posts p
WHERE p.deleted_at IS NULL
  AND p.hidden_at IS NULL
  AND (sqlc.narg('author_id')::uuid IS NULL OR p.author_id = sqlc.narg('author_id'))
  AND (sqlc.narg('popular_window')::bool IS NOT TRUE OR p.created_at >= now() - interval '14 days')
  AND (
    sqlc.narg('keyword')::text IS NULL
    OR p.title ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
    OR p.body ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
  );

-- name: FindYouTubePostID :one
SELECT id
FROM posts
WHERE youtube_video_id = $1
  AND deleted_at IS NULL;

-- name: GetPost :one
SELECT
    p.id,
    p.author_id,
    p.title,
    p.body,
    p.kind,
    p.youtube_video_id,
    p.youtube_thumbnail_url,
    p.cloudinary_public_id,
    p.like_count,
    p.comment_count,
    p.created_at,
    p.updated_at
FROM posts p
WHERE p.id = $1
  AND p.deleted_at IS NULL
  AND p.hidden_at IS NULL;

-- name: CreatePost :one
INSERT INTO posts (
    id, author_id, title, body,
    kind, youtube_video_id, youtube_thumbnail_url, cloudinary_public_id
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, author_id, title, body, kind, youtube_video_id, youtube_thumbnail_url, cloudinary_public_id, like_count, comment_count, created_at, updated_at;

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
    p.cloudinary_public_id,
    p.like_count,
    p.comment_count,
    p.created_at,
    p.updated_at
FROM post_saves s
JOIN posts p ON p.id = s.post_id
WHERE s.user_id = sqlc.arg('user_id')
  AND p.deleted_at IS NULL
  AND p.hidden_at IS NULL
  AND (
    sqlc.narg('keyword')::text IS NULL
    OR p.title ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
    OR p.body ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
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
  AND (
    sqlc.narg('keyword')::text IS NULL
    OR p.title ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
    OR p.body ILIKE '%' || sqlc.narg('keyword') || '%' ESCAPE '\'
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
UPDATE posts SET like_count = like_count + 1, updated_at = now() WHERE id = $1;

-- name: DecrementLikeCount :exec
UPDATE posts SET like_count = GREATEST(like_count - 1, 0), updated_at = now() WHERE id = $1;

-- name: IncrementCommentCount :exec
UPDATE posts SET comment_count = comment_count + 1, updated_at = now() WHERE id = $1;

-- name: DecrementCommentCount :exec
UPDATE posts SET comment_count = GREATEST(comment_count - 1, 0), updated_at = now() WHERE id = $1;
