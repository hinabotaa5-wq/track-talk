-- name: PledgeCoach :one
-- パーソナル指導の誓約。承認済みのコーチだけ。文面が変わったら版を上げて、もう一度誓約してもらう。
UPDATE coach_profiles
SET pledged_at = now(),
    pledge_version = sqlc.arg(pledge_version)::smallint,
    updated_at = now()
WHERE profile_id = sqlc.arg(profile_id)
  AND status = 'approved'
RETURNING *;

-- name: CountCoachingPlans :one
SELECT count(*) FROM coaching_plans
WHERE coach_id = $1;

-- name: CreateCoachingPlan :one
INSERT INTO coaching_plans (id, coach_id, kind, title, description, duration_days, price_yen, capacity, published)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetCoachingPlan :one
SELECT * FROM coaching_plans
WHERE id = $1 AND coach_id = $2;

-- name: UpdateCoachingPlan :one
UPDATE coaching_plans
SET kind = $3,
    title = $4,
    description = $5,
    duration_days = $6,
    price_yen = $7,
    capacity = $8,
    published = $9,
    updated_at = now()
WHERE id = $1 AND coach_id = $2
RETURNING *;

-- name: ListMyCoachingPlans :many
SELECT * FROM coaching_plans
WHERE coach_id = $1
ORDER BY created_at, id;

-- name: ListPublicCoachingPlans :many
-- コーチ一覧に出すプラン。枠を使っている人数（支払い待ち・期間中・停止中）をつけて、満員かどうかを出す。承認済みで、いまの版の誓約をしたコーチの、公開中のものだけ。
SELECT pl.*,
    (SELECT count(*) FROM coaching_contracts ct WHERE ct.plan_id = pl.id AND ct.status IN ('accepted', 'active', 'suspended'))::int AS active_count
FROM coaching_plans pl
JOIN coach_profiles c ON c.profile_id = pl.coach_id
WHERE pl.published
  AND c.status = 'approved'
  AND c.pledge_version = sqlc.arg(pledge_version)::smallint
ORDER BY pl.coach_id, pl.price_yen, pl.created_at, pl.id;

-- name: LockCoachingPlans :exec
-- 同じコーチのプランの作成を1つずつにする。10個の上限の確認と作成のあいだに、別の作成を割り込ませない。
SELECT pg_advisory_xact_lock(hashtext('coaching_plans:' || sqlc.arg(coach_id)::uuid::text));

-- name: CountOtherTrialPlans :one
-- このコーチの、ほかのお試しプランの数。お試しプランは1人1つまで。
SELECT count(*) FROM coaching_plans
WHERE coach_id = sqlc.arg(coach_id) AND kind = 'trial' AND id <> sqlc.arg(exclude_id)::uuid;
