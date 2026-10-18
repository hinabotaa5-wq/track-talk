-- name: ListDomesticMajors :many
-- シーズン [season_start, season_end) の中で始まるすべての回。
SELECT slug, year, name, category, venue, starts_on, ends_on, schedule_label, site_url
FROM domestic_majors
WHERE starts_on >= sqlc.arg(season_start)::date
  AND starts_on < sqlc.arg(season_end)::date
ORDER BY starts_on ASC, slug ASC;

-- name: UpsertDomesticMajor :exec
INSERT INTO domestic_majors (
    slug, year, name, category, venue, starts_on, ends_on, schedule_label, site_url
) VALUES (
    sqlc.arg(slug), sqlc.arg(year), sqlc.arg(name), sqlc.arg(category), sqlc.arg(venue),
    sqlc.arg(starts_on), sqlc.arg(ends_on), sqlc.arg(schedule_label), sqlc.arg(site_url)
)
ON CONFLICT (slug, year) DO UPDATE
SET name = EXCLUDED.name,
    category = EXCLUDED.category,
    venue = EXCLUDED.venue,
    starts_on = EXCLUDED.starts_on,
    ends_on = EXCLUDED.ends_on,
    schedule_label = EXCLUDED.schedule_label,
    site_url = EXCLUDED.site_url,
    updated_at = now();
