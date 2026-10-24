-- name: DailyUsageCollectionStart :one
SELECT started_at FROM daily_usage_collection WHERE singleton;

-- name: AdminDailyUsage :many
-- データがない日も0を返す。開始日以前は呼び出し側で除外する。
WITH days AS (
    SELECT generate_series(sqlc.arg(since)::date::timestamp,
        sqlc.arg(until)::date::timestamp, interval '1 day')::date AS day
)
SELECT d.day,
    COALESCE(sum(c.count) FILTER (WHERE c.metric = 'signup'), 0)::bigint AS signups,
    COALESCE(sum(c.count) FILTER (WHERE c.metric = 'post'), 0)::bigint AS posts,
    COALESCE(sum(c.count) FILTER (WHERE c.metric = 'comment'), 0)::bigint AS comments,
    COALESCE(sum(c.count) FILTER (WHERE c.metric = 'like'), 0)::bigint AS likes,
    COALESCE(sum(c.count) FILTER (WHERE c.metric = 'sense_note'), 0)::bigint AS sense_notes,
    COALESCE(sum(c.count) FILTER (WHERE c.metric = 'race_record'), 0)::bigint AS race_records,
    COALESCE(sum(c.count) FILTER (WHERE c.metric = 'product_review'), 0)::bigint AS product_reviews,
    COALESCE(sum(c.count) FILTER (WHERE c.metric = 'deletion'), 0)::bigint AS deletions
FROM days d
LEFT JOIN daily_usage_counts c ON c.day = d.day
GROUP BY d.day
ORDER BY d.day DESC;
