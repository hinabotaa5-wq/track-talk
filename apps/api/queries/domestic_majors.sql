-- name: ListDomesticMajors :many
-- シーズン [season_start, season_end) にかかるすべての回。3月に始まって4月にまたがる回も、4月になって開催中なら含める。
SELECT slug, year, name, category, venue, starts_on, ends_on, schedule_label, site_url
FROM domestic_majors
WHERE starts_on < sqlc.arg(season_end)::date
  AND COALESCE(ends_on, starts_on) >= sqlc.arg(season_start)::date
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
