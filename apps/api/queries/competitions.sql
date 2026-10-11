-- name: ListCompetitions :many
SELECT id, name, starts_on, ends_on, location, scope, summary, created_at, updated_at
FROM competitions
ORDER BY starts_on DESC
LIMIT $1 OFFSET $2;

-- name: CountCompetitions :one
SELECT count(*) FROM competitions;

-- name: GetCompetition :one
SELECT id, name, starts_on, ends_on, location, scope, summary, created_at, updated_at
FROM competitions
WHERE id = $1;

-- name: ListCategories :many
SELECT id, name, sort_order
FROM post_categories
ORDER BY sort_order ASC;
