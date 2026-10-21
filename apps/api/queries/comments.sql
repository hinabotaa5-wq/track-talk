-- name: ListComments :many
SELECT
    c.id,
    c.post_id,
    c.author_id,
    c.body,
    c.created_at,
    c.updated_at,
    c.parent_id,
    c.reply_to_id,
    t.author_id AS reply_to_author_id,
    COALESCE(t.deleted_at IS NOT NULL OR t.hidden_at IS NOT NULL, false)::boolean AS reply_to_removed
FROM comments c
LEFT JOIN comments t ON t.id = c.reply_to_id
WHERE c.post_id = $1
  AND c.deleted_at IS NULL
  AND c.hidden_at IS NULL
ORDER BY c.created_at ASC
LIMIT $2 OFFSET $3;

-- name: CountComments :one
SELECT count(*)
FROM comments
WHERE post_id = $1
  AND deleted_at IS NULL
  AND hidden_at IS NULL;

-- name: GetComment :one
SELECT id, post_id, author_id, body, created_at, updated_at, parent_id, reply_to_id
FROM comments
WHERE id = $1
  AND deleted_at IS NULL
  AND hidden_at IS NULL;

-- name: CreateComment :one
INSERT INTO comments (id, post_id, author_id, body, parent_id, reply_to_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, post_id, author_id, body, created_at, updated_at, parent_id, reply_to_id;

-- name: UpdateComment :one
UPDATE comments
SET body = $3, updated_at = now()
WHERE id = $1
  AND author_id = $2
  AND deleted_at IS NULL
  AND hidden_at IS NULL
RETURNING id, post_id, author_id, body, created_at, updated_at, parent_id, reply_to_id;

-- name: SoftDeleteComment :execrows
UPDATE comments
SET deleted_at = now(), updated_at = now()
WHERE id = $1
  AND author_id = $2
  AND deleted_at IS NULL;

-- name: HideComment :execrows
UPDATE comments
SET hidden_at = now(), updated_at = now()
WHERE id = $1
  AND deleted_at IS NULL
  AND hidden_at IS NULL;
