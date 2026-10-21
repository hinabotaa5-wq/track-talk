-- 管理者画面の「数字」タブ。どれも件数と人数だけを返し、本文やアカウントは返さない。
-- 週は日本時間の月曜始まり。退会した人の行は消えているので、過去の週の数字も退会のぶん減る。

-- name: AdminWeeklyStats :many
-- 書き込み(投稿・コメント・いいね・感覚メモ・記録・レビュー・練習の申請・ケガのグループへの参加)をした人を「動いた人」と数える。
-- 読むだけの人は数えられないので、ページを開いた人数は Umami で見る。
WITH weeks AS (
    SELECT generate_series(
        date_trunc('week', sqlc.arg(since)::timestamptz AT TIME ZONE 'Asia/Tokyo'),
        date_trunc('week', sqlc.arg(until)::timestamptz AT TIME ZONE 'Asia/Tokyo'),
        interval '1 week'
    ) AS week_start
),
activity AS (
    SELECT author_id AS user_id, created_at FROM posts WHERE kind <> 'meet'
    UNION ALL SELECT author_id, created_at FROM comments
    UNION ALL SELECT user_id, created_at FROM post_likes
    UNION ALL SELECT author_id, created_at FROM sense_notes
    UNION ALL SELECT author_id, created_at FROM race_records
    UNION ALL SELECT author_id, created_at FROM product_reviews
    UNION ALL SELECT user_id, created_at FROM practice_applications
    UNION ALL SELECT user_id, joined_at FROM injury_group_members
)
SELECT
    w.week_start::date AS week_start,
    (SELECT count(*) FROM profiles p
        WHERE p.created_at AT TIME ZONE 'Asia/Tokyo' >= w.week_start
          AND p.created_at AT TIME ZONE 'Asia/Tokyo' < w.week_start + interval '1 week')::bigint AS signups,
    (SELECT count(DISTINCT a.user_id) FROM activity a
        WHERE a.created_at AT TIME ZONE 'Asia/Tokyo' >= w.week_start
          AND a.created_at AT TIME ZONE 'Asia/Tokyo' < w.week_start + interval '1 week')::bigint AS active_users,
    (SELECT count(*) FROM posts x
        WHERE x.kind <> 'meet'
          AND x.created_at AT TIME ZONE 'Asia/Tokyo' >= w.week_start
          AND x.created_at AT TIME ZONE 'Asia/Tokyo' < w.week_start + interval '1 week')::bigint AS posts,
    (SELECT count(*) FROM sense_notes x
        WHERE x.created_at AT TIME ZONE 'Asia/Tokyo' >= w.week_start
          AND x.created_at AT TIME ZONE 'Asia/Tokyo' < w.week_start + interval '1 week')::bigint AS sense_notes,
    (SELECT count(*) FROM race_records x
        WHERE x.created_at AT TIME ZONE 'Asia/Tokyo' >= w.week_start
          AND x.created_at AT TIME ZONE 'Asia/Tokyo' < w.week_start + interval '1 week')::bigint AS race_records,
    (SELECT count(*) FROM deleted_accounts x
        WHERE x.deleted_at AT TIME ZONE 'Asia/Tokyo' >= w.week_start
          AND x.deleted_at AT TIME ZONE 'Asia/Tokyo' < w.week_start + interval '1 week')::bigint AS deletions
FROM weeks w
ORDER BY w.week_start DESC;

-- name: AdminRetention :many
-- 登録した週ごとに、登録から29〜35日目(5週目)にも書き込みをした人の割合。
-- 5週目が終わっていない週は、まだ数えられないので返さない。
WITH cohorts AS (
    SELECT p.id, p.created_at, date_trunc('week', p.created_at AT TIME ZONE 'Asia/Tokyo') AS week_start
    FROM profiles p
    WHERE p.created_at >= sqlc.arg(since)::timestamptz
      AND p.created_at + interval '35 days' <= sqlc.arg(now)::timestamptz
),
activity AS (
    SELECT author_id AS user_id, created_at FROM posts WHERE kind <> 'meet'
    UNION ALL SELECT author_id, created_at FROM comments
    UNION ALL SELECT user_id, created_at FROM post_likes
    UNION ALL SELECT author_id, created_at FROM sense_notes
    UNION ALL SELECT author_id, created_at FROM race_records
    UNION ALL SELECT author_id, created_at FROM product_reviews
    UNION ALL SELECT user_id, created_at FROM practice_applications
    UNION ALL SELECT user_id, joined_at FROM injury_group_members
)
SELECT
    c.week_start::date AS week_start,
    count(*)::bigint AS signups,
    count(*) FILTER (WHERE EXISTS (
        SELECT 1 FROM activity a
        WHERE a.user_id = c.id
          AND a.created_at >= c.created_at + interval '28 days'
          AND a.created_at < c.created_at + interval '35 days'
    ))::bigint AS retained
FROM cohorts c
GROUP BY c.week_start
ORDER BY c.week_start DESC;

-- name: AdminFeatureUsage :many
-- 機能ごとの、直近の期間の件数と使った人数、これまでの件数。
WITH usage AS (
    SELECT 'post:' || kind AS feature, author_id AS user_id, created_at FROM posts WHERE kind <> 'meet'
    UNION ALL SELECT 'comment', author_id, created_at FROM comments
    UNION ALL SELECT 'like', user_id, created_at FROM post_likes
    UNION ALL SELECT 'save', user_id, created_at FROM post_saves
    UNION ALL SELECT 'sense_note', author_id, created_at FROM sense_notes
    UNION ALL SELECT 'race_record', author_id, created_at FROM race_records
    UNION ALL SELECT 'product_review', author_id, created_at FROM product_reviews WHERE deleted_at IS NULL
    UNION ALL SELECT 'practice_application', user_id, created_at FROM practice_applications
    UNION ALL SELECT 'injury_group', user_id, joined_at FROM injury_group_members
    UNION ALL SELECT 'menu_contest_entry', author_id, created_at FROM menu_contest_entries
    UNION ALL SELECT 'menu_contest_vote', voter_id, created_at FROM menu_contest_votes
)
SELECT
    u.feature::text AS feature,
    count(*) FILTER (WHERE u.created_at >= sqlc.arg(since)::timestamptz)::bigint AS recent,
    count(DISTINCT u.user_id) FILTER (WHERE u.created_at >= sqlc.arg(since)::timestamptz)::bigint AS recent_users,
    count(*)::bigint AS total
FROM usage u
GROUP BY u.feature
ORDER BY recent DESC, total DESC, feature;

-- name: AdminInquiryBreakdown :many
SELECT
    source,
    category,
    count(*) FILTER (WHERE created_at >= sqlc.arg(since)::timestamptz)::bigint AS recent,
    count(*)::bigint AS total
FROM inquiries
GROUP BY source, category
ORDER BY recent DESC, total DESC, source, category;

-- name: AdminReportBreakdown :many
SELECT
    target_type,
    reason,
    count(*) FILTER (WHERE created_at >= sqlc.arg(since)::timestamptz)::bigint AS recent,
    count(*) FILTER (WHERE status = 'open')::bigint AS open,
    count(*)::bigint AS total
FROM reports
GROUP BY target_type, reason
ORDER BY recent DESC, total DESC, target_type, reason;

-- name: AdminCoachSupply :many
-- 承認済みのコーチが担当する種目ごとの人数。1人が複数の種目を持つので、合計は人数より多くなる。
SELECT
    e.event::text AS event,
    count(*)::bigint AS coaches,
    count(*) FILTER (WHERE c.on_duty)::bigint AS on_duty
FROM coach_profiles c
CROSS JOIN LATERAL unnest(c.events) AS e(event)
WHERE c.status = 'approved'
GROUP BY e.event
ORDER BY coaches DESC, event;

-- name: AdminReviewDemand :many
-- 期間内に出たフォーム添削の依頼を種目ごとに。返すまでの時間は、返した依頼だけの中央値。
SELECT
    r.event,
    count(*)::bigint AS requests,
    count(*) FILTER (WHERE r.returned_at IS NOT NULL)::bigint AS returned,
    count(*) FILTER (WHERE r.status = 'expired')::bigint AS expired,
    COALESCE(percentile_cont(0.5) WITHIN GROUP (ORDER BY extract(epoch FROM r.returned_at - r.created_at) / 3600)
        FILTER (WHERE r.returned_at IS NOT NULL), 0)::float8 AS median_hours,
    count(g.stars)::bigint AS rated,
    COALESCE(avg(g.stars), 0)::float8 AS avg_stars,
    count(g.helpful)::bigint AS helpful_answers,
    count(*) FILTER (WHERE g.helpful = 'yes')::bigint AS helpful_yes
FROM review_requests r
LEFT JOIN review_ratings g ON g.request_id = r.id
WHERE r.created_at >= sqlc.arg(since)::timestamptz
GROUP BY r.event
ORDER BY requests DESC, r.event;

-- name: AdminContractDemand :many
-- 期間内に申し込まれたパーソナル指導を種目ごとに。始まった(期間中・終了)、断られた・期限切れ・取り消しを分けて数える。
SELECT
    c.event,
    count(*)::bigint AS applied,
    count(*) FILTER (WHERE c.status IN ('active', 'ended'))::bigint AS started,
    count(*) FILTER (WHERE c.status IN ('declined', 'expired', 'cancelled'))::bigint AS lost,
    COALESCE(sum(c.price_yen) FILTER (WHERE c.status IN ('active', 'ended')), 0)::bigint AS started_yen
FROM coaching_contracts c
WHERE c.created_at >= sqlc.arg(since)::timestamptz
GROUP BY c.event
ORDER BY applied DESC, c.event;
