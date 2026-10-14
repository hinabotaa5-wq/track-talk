-- name: ListCompetitions :many
SELECT id, name, starts_on, ends_on, location, created_at, updated_at, prefecture, season_year, organizer_url, source_url, link_label, guidelines_url
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

-- name: DeleteCompetitionsBeforeSeason :exec
DELETE FROM competitions
WHERE prefecture = sqlc.arg(prefecture)
  AND season_year < sqlc.arg(season_year);

-- name: DeletePastCompetitionSeasons :execrows
DELETE FROM competitions
WHERE season_year < sqlc.arg(season_year);

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


-- name: DeleteCompetitionsBySeasonExcept :exec
-- 取り込んだ大会に含まれない行だけを消す。残る行は upsert で更新され、新着から補った要項が消えない。
DELETE FROM competitions c
WHERE c.prefecture = sqlc.arg(prefecture)
  AND c.season_year = sqlc.arg(season_year)
  AND (c.name || '|' || to_char(c.starts_on, 'YYYY-MM-DD')) <> ALL (sqlc.arg(keys)::text[]);

-- name: ListCompetitionsForNews :many
SELECT id, name, starts_on, ends_on, location, link_label, organizer_url, source_url, guidelines_url
FROM competitions
WHERE prefecture = sqlc.arg(prefecture)
  AND season_year = sqlc.arg(season_year)
ORDER BY starts_on ASC, name ASC;

-- name: SetCompetitionGuidelines :exec
-- 要項の補完は年間日程の取り込み時刻(updated_at)を動かさない。まだ要項が無い行にだけ入れる。
UPDATE competitions
SET guidelines_url = sqlc.arg(guidelines_url)
WHERE id = sqlc.arg(id)
  AND guidelines_url = '';

-- name: ListNewsCheckTimes :many
SELECT prefecture, checked_at FROM competition_news_checks;

-- name: MarkNewsChecked :exec
INSERT INTO competition_news_checks (prefecture, checked_at)
VALUES (sqlc.arg(prefecture), now())
ON CONFLICT (prefecture) DO UPDATE SET checked_at = now();
