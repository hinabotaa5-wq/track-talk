-- name: ListCompetitions :many
SELECT id, name, starts_on, ends_on, location, created_at, updated_at, prefecture, season_year, organizer_url, source_url, link_label
FROM competitions
WHERE prefecture = sqlc.arg(prefecture)
  AND starts_on < sqlc.arg(month_end)::date
  AND (
    (ends_on IS NULL AND starts_on >= sqlc.arg(month_start)::date)
    OR ends_on >= sqlc.arg(month_start)::date
  )
ORDER BY starts_on ASC, name ASC;

-- name: DeleteCompetitionsBySeason :exec
DELETE FROM competitions
WHERE prefecture = sqlc.arg(prefecture)
  AND season_year = sqlc.arg(season_year);

-- name: UpsertCompetition :exec
INSERT INTO competitions (
    id, name, starts_on, ends_on, location, prefecture, season_year, organizer_url, source_url, link_label
) VALUES (
    sqlc.arg(id), sqlc.arg(name), sqlc.arg(starts_on), sqlc.narg(ends_on), sqlc.arg(location),
    sqlc.arg(prefecture), sqlc.arg(season_year), sqlc.arg(organizer_url), sqlc.arg(source_url), sqlc.arg(link_label)
)
ON CONFLICT (prefecture, season_year, name, starts_on) DO UPDATE
SET ends_on = EXCLUDED.ends_on,
    location = EXCLUDED.location,
    organizer_url = EXCLUDED.organizer_url,
    source_url = EXCLUDED.source_url,
    link_label = EXCLUDED.link_label,
    updated_at = now();

