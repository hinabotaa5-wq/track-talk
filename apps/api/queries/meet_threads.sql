-- name: GetMeetThreadPostID :one
SELECT post_id
FROM meet_threads
WHERE meet_key = $1;

-- name: CreateMeetPost :exec
INSERT INTO posts (id, author_id, title, body, kind)
VALUES ($1, $2, $3, $4, 'meet');

-- name: InsertMeetThread :exec
INSERT INTO meet_threads (meet_key, post_id)
VALUES ($1, $2);

-- name: ListMeetThreads :many
SELECT
    mt.meet_key,
    p.id AS post_id,
    p.title,
    p.comment_count
FROM meet_threads mt
JOIN posts p ON p.id = mt.post_id
WHERE mt.meet_key = ANY(sqlc.arg('meet_keys')::text[])
  AND p.deleted_at IS NULL
  AND p.hidden_at IS NULL;
