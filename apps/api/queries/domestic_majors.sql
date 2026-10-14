-- name: ListDomesticMajors :many
-- One edition per meet: the next one that has not ended, else the latest one that has.
SELECT DISTINCT ON (slug) slug, year, name, category, venue, starts_on, ends_on, schedule_label, site_url
FROM domestic_majors
ORDER BY slug,
    (ends_on < sqlc.arg(today)::date),
    CASE WHEN ends_on >= sqlc.arg(today)::date THEN starts_on END ASC,
    ends_on DESC;

-- name: DeleteDomesticMajorsBySlug :exec
DELETE FROM domestic_majors
WHERE slug = sqlc.arg(slug);

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
