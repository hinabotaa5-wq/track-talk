-- name: ListCompetitions :many
SELECT id, name, starts_on, ends_on, location, created_at, updated_at, prefecture, season_year, organizer_url, source_url
FROM competitions
WHERE prefecture = sqlc.arg(prefecture)
  AND starts_on < sqlc.arg(month_end)::date
  AND (
    (ends_on IS NULL AND starts_on >= sqlc.arg(month_start)::date)
    OR ends_on >= sqlc.arg(month_start)::date
  )
ORDER BY starts_on ASC, name ASC;
