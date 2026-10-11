-- name: ListPostsNew :many
SELECT
    p.id,
    p.author_id,
    p.category_id,
    p.competition_id,
    p.title,
    p.body,
    p.like_count,
    p.comment_count,
    p.created_at,
    p.updated_at,
    pr.nickname AS author_nickname,
    cat.name AS category_name,
    comp.name AS competition_name
FROM posts p
JOIN profiles pr ON pr.id = p.author_id
JOIN post_categories cat ON cat.id = p.category_id
LEFT JOIN competitions comp ON comp.id = p.competition_id
WHERE p.deleted_at IS NULL
  AND p.hidden_at IS NULL
  AND (sqlc.narg('category_id')::text IS NULL OR p.category_id = sqlc.narg('category_id'))
  AND (sqlc.narg('competition_id')::uuid IS NULL OR p.competition_id = sqlc.narg('competition_id'))
  AND (sqlc.narg('author_id')::uuid IS NULL OR p.author_id = sqlc.narg('author_id'))
ORDER BY p.created_at DESC
LIMIT sqlc.arg('limit_n') OFFSET sqlc.arg('offset_n');

-- name: ListPostsPopular :many
SELECT
    p.id,
    p.author_id,
    p.category_id,
    p.competition_id,
    p.title,
    p.body,
    p.like_count,
    p.comment_count,
    p.created_at,
    p.updated_at,
    pr.nickname AS author_nickname,
    cat.name AS category_name,
    comp.name AS competition_name
FROM posts p
JOIN profiles pr ON pr.id = p.author_id
JOIN post_categories cat ON cat.id = p.category_id
LEFT JOIN competitions comp ON comp.id = p.competition_id
WHERE p.deleted_at IS NULL
  AND p.hidden_at IS NULL
  AND p.created_at >= now() - interval '30 days'
  AND (sqlc.narg('category_id')::text IS NULL OR p.category_id = sqlc.narg('category_id'))
  AND (sqlc.narg('competition_id')::uuid IS NULL OR p.competition_id = sqlc.narg('competition_id'))
  AND (sqlc.narg('author_id')::uuid IS NULL OR p.author_id = sqlc.narg('author_id'))
ORDER BY p.like_count DESC, p.created_at DESC
LIMIT sqlc.arg('limit_n') OFFSET sqlc.arg('offset_n');

-- name: CountPosts :one
SELECT count(*)
FROM posts p
WHERE p.deleted_at IS NULL
  AND p.hidden_at IS NULL
  AND (sqlc.narg('category_id')::text IS NULL OR p.category_id = sqlc.narg('category_id'))
  AND (sqlc.narg('competition_id')::uuid IS NULL OR p.competition_id = sqlc.narg('competition_id'))
  AND (sqlc.narg('author_id')::uuid IS NULL OR p.author_id = sqlc.narg('author_id'))
  AND (sqlc.narg('popular_window')::bool IS NOT TRUE OR p.created_at >= now() - interval '30 days');

-- name: GetPost :one
SELECT
    p.id,
    p.author_id,
    p.category_id,
    p.competition_id,
    p.title,
    p.body,
    p.like_count,
    p.comment_count,
    p.created_at,
    p.updated_at,
    pr.nickname AS author_nickname,
    cat.name AS category_name,
    comp.name AS competition_name
FROM posts p
JOIN profiles pr ON pr.id = p.author_id
JOIN post_categories cat ON cat.id = p.category_id
LEFT JOIN competitions comp ON comp.id = p.competition_id
WHERE p.id = $1
  AND p.deleted_at IS NULL
  AND p.hidden_at IS NULL;

-- name: CreatePost :one
INSERT INTO posts (id, author_id, category_id, competition_id, title, body)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, author_id, category_id, competition_id, title, body, like_count, comment_count, created_at, updated_at;

-- name: UpdatePost :one
UPDATE posts
SET category_id = $3,
    competition_id = $4,
    title = $5,
    body = $6,
    updated_at = now()
WHERE id = $1
  AND author_id = $2
  AND deleted_at IS NULL
  AND hidden_at IS NULL
RETURNING id, author_id, category_id, competition_id, title, body, like_count, comment_count, created_at, updated_at;

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
