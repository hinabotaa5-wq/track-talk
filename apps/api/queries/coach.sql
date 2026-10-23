-- name: GetCoachProfile :one
SELECT * FROM coach_profiles
WHERE profile_id = $1;

-- name: SubmitCoachApplication :one
-- 審査待ちのうちは書き直せる。却下されたら出し直せて、また審査待ちに戻る。
-- 承認済み・停止中は変えられない（行が返らない）。
INSERT INTO coach_profiles (profile_id, tier, events, best_record, achievements, years)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (profile_id) DO UPDATE
SET tier = EXCLUDED.tier,
    events = EXCLUDED.events,
    best_record = EXCLUDED.best_record,
    achievements = EXCLUDED.achievements,
    years = EXCLUDED.years,
    status = 'pending',
    applied_at = now(),
    reviewed_at = NULL,
    updated_at = now()
WHERE coach_profiles.status IN ('pending', 'rejected')
RETURNING *;

-- name: CountCoachApplications :one
SELECT count(*) FROM coach_profiles
WHERE status = $1;

-- name: ListCoachApplications :many
SELECT
    c.profile_id,
    c.tier,
    c.events,
    c.best_record,
    c.achievements,
    c.years,
    c.status,
    c.applied_at,
    c.reviewed_at,
    p.nickname,
    c.photo_public_id,
    r.rating_count,
    r.rating_average
FROM coach_profiles c
JOIN profiles p ON p.id = c.profile_id
CROSS JOIN LATERAL (
    -- 打診の順番から外す判定に使う★（運営が戻したあとのものだけ）。
    SELECT count(rt.stars)::int AS rating_count, coalesce(avg(rt.stars), 0)::float8 AS rating_average
    FROM review_ratings rt
    JOIN review_responses rs ON rs.request_id = rt.request_id
    WHERE rs.coach_id = c.profile_id
      AND rt.rated_at > coalesce(c.rating_reset_at, '-infinity')
) r
WHERE c.status = $1
ORDER BY c.applied_at ASC
LIMIT $2 OFFSET $3;

-- name: ReviewCoachApplication :one
-- 審査待ちのものだけ承認・却下できる。顔写真がないと承認できない。
UPDATE coach_profiles
SET status = sqlc.arg(status),
    reviewed_at = now(),
    updated_at = now()
WHERE profile_id = sqlc.arg(profile_id)
  AND status = 'pending'
  AND (sqlc.arg(status)::text <> 'approved' OR photo_public_id IS NOT NULL)
RETURNING *;

-- name: ResetCoachRating :one
-- 評価で順番から外れたコーチを戻す。これより前の★は数えなくなる。
UPDATE coach_profiles
SET rating_reset_at = now(),
    updated_at = now()
WHERE profile_id = $1
  AND status = 'approved'
RETURNING *;

-- name: ListPublicCoaches :many
-- 依頼する前に見られるコーチ一覧。承認されたコーチだけ。★はこれまでの全部(添削とパーソナル指導)。
SELECT
    c.profile_id,
    p.nickname,
    c.photo_public_id,
    c.tier,
    c.events,
    c.best_record,
    c.achievements,
    c.years,
    c.service_types,
    r.rating_count,
    r.rating_average
FROM coach_profiles c
JOIN profiles p ON p.id = c.profile_id
CROSS JOIN LATERAL (
    -- 添削の★とパーソナル指導の★を合わせて数える。打診の順番から外す判定(SelectReviewCoach など)は添削の★だけを使う。
    SELECT count(x.stars)::int AS rating_count, coalesce(avg(x.stars), 0)::float8 AS rating_average
    FROM (
        SELECT rt.stars
        FROM review_ratings rt
        JOIN review_responses rs ON rs.request_id = rt.request_id
        WHERE rs.coach_id = c.profile_id
        UNION ALL
        SELECT cr.stars
        FROM coaching_contract_ratings cr
        JOIN coaching_contracts ct ON ct.id = cr.contract_id
        WHERE ct.coach_id = c.profile_id
    ) x
) r
WHERE c.status = 'approved'
ORDER BY c.tier, c.reviewed_at, c.profile_id;

-- name: SetCoachPhoto :one
-- 顔写真を差し替える。前の写真の ID を返すので、Cloudinary から消す。
UPDATE coach_profiles c
SET photo_public_id = sqlc.arg(photo_public_id)::text,
    updated_at = now()
FROM (SELECT photo_public_id AS old_photo_public_id FROM coach_profiles WHERE profile_id = sqlc.arg(profile_id)) o
WHERE c.profile_id = sqlc.arg(profile_id)
  AND c.status IN ('pending', 'approved')
RETURNING o.old_photo_public_id;

-- name: RemoveCoachPhoto :one
-- 運営が顔写真を外す。外した写真の ID を返すので、Cloudinary から消す。
UPDATE coach_profiles c
SET photo_public_id = NULL,
    updated_at = now()
FROM (SELECT photo_public_id AS old_photo_public_id FROM coach_profiles WHERE profile_id = $1) o
WHERE c.profile_id = $1
RETURNING o.old_photo_public_id;

-- name: UpdateCoachServices :one
UPDATE coach_profiles
SET service_types = sqlc.arg(service_types)::text[], updated_at = now()
WHERE profile_id = sqlc.arg(profile_id) AND status = 'approved'
RETURNING *;

-- name: HasPublishedPlanOutsideServices :one
SELECT EXISTS (
    SELECT 1 FROM coaching_plans
    WHERE coach_id = sqlc.arg(coach_id) AND published AND service_type IS NOT NULL
      AND NOT (service_type = ANY(sqlc.arg(service_types)::text[]))
)::boolean;
