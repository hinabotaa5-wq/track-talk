-- name: ListComments :many
SELECT
    c.id,
    c.post_id,
    c.author_id,
    c.body,
    c.created_at,
    c.updated_at
FROM comments c
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
SELECT id, post_id, author_id, body, created_at, updated_at
FROM comments
WHERE id = $1
  AND deleted_at IS NULL
  AND hidden_at IS NULL;

-- name: CreateComment :one
INSERT INTO comments (id, post_id, author_id, body)
VALUES ($1, $2, $3, $4)
RETURNING id, post_id, author_id, body, created_at, updated_at;

-- name: UpdateComment :one
UPDATE comments
SET body = $3, updated_at = now()
WHERE id = $1
  AND author_id = $2
  AND deleted_at IS NULL
  AND hidden_at IS NULL
RETURNING id, post_id, author_id, body, created_at, updated_at;

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
