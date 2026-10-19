-- name: ListUpcomingMajorHosts :many
SELECT kind, year, name, city, country, venue, starts_on, ends_on, schedule_label, image_url, page_url
FROM major_hosts
WHERE ends_on >= sqlc.arg(today)::date
ORDER BY starts_on ASC, name ASC;

-- name: DeleteMajorHostsNotInYears :exec
-- 今回の取り込みになかった年の行だけ消す（ある年の行は UpsertMajorHost で更新する）。
DELETE FROM major_hosts
WHERE kind = sqlc.arg(kind)
  AND year <> ALL (sqlc.arg(years)::int[]);

-- name: UpsertMajorHost :exec
INSERT INTO major_hosts (
    kind, year, name, city, country, venue, starts_on, ends_on, schedule_label, image_url, page_url
) VALUES (
    sqlc.arg(kind), sqlc.arg(year), sqlc.arg(name), sqlc.arg(city), sqlc.arg(country), sqlc.arg(venue),
    sqlc.arg(starts_on), sqlc.arg(ends_on), sqlc.arg(schedule_label), sqlc.arg(image_url), sqlc.arg(page_url)
)
ON CONFLICT (kind, year) DO UPDATE
SET name = EXCLUDED.name,
    city = EXCLUDED.city,
    country = EXCLUDED.country,
    venue = EXCLUDED.venue,
    starts_on = EXCLUDED.starts_on,
    ends_on = EXCLUDED.ends_on,
    schedule_label = EXCLUDED.schedule_label,
    -- 写真が取れなかったとき（Wikimedia の一時的な失敗など）は、同じ開催地の前回の写真を残す。
    image_url = CASE
        WHEN EXCLUDED.image_url <> '' THEN EXCLUDED.image_url
        WHEN major_hosts.city = EXCLUDED.city THEN major_hosts.image_url
        ELSE ''
    END,
    page_url = EXCLUDED.page_url,
    updated_at = now();
