-- name: ListCompetitions :many
SELECT id, name, starts_on, ends_on, location, scope, summary, created_at, updated_at
FROM competitions
WHERE ends_on >= (now() AT TIME ZONE 'Asia/Tokyo')::date - 30
   OR (ends_on IS NULL AND starts_on >= (now() AT TIME ZONE 'Asia/Tokyo')::date - 30)
ORDER BY starts_on ASC
LIMIT $1 OFFSET $2;

-- name: CountCompetitions :one
SELECT count(*) FROM competitions
WHERE ends_on >= (now() AT TIME ZONE 'Asia/Tokyo')::date - 30
   OR (ends_on IS NULL AND starts_on >= (now() AT TIME ZONE 'Asia/Tokyo')::date - 30);

-- name: GetCompetition :one
SELECT id, name, starts_on, ends_on, location, scope, summary, created_at, updated_at
FROM competitions
WHERE id = $1;
