-- name: GetCoachProfile :one
SELECT * FROM coach_profiles
WHERE profile_id = $1;

-- name: SubmitCoachApplication :one
-- 審査待ちのうちは書き直せる。却下されたら出し直せて、また審査待ちに戻る。
-- 承認済み・停止中は変えられない（行が返らない）。
INSERT INTO coach_profiles (profile_id, tier, event_groups, best_record, achievements, years)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (profile_id) DO UPDATE
SET tier = EXCLUDED.tier,
    event_groups = EXCLUDED.event_groups,
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
    c.event_groups,
    c.best_record,
    c.achievements,
    c.years,
    c.status,
    c.applied_at,
    c.reviewed_at,
    p.nickname
FROM coach_profiles c
JOIN profiles p ON p.id = c.profile_id
WHERE c.status = $1
ORDER BY c.applied_at ASC
LIMIT $2 OFFSET $3;

-- name: ReviewCoachApplication :one
-- 審査待ちのものだけ承認・却下できる。
UPDATE coach_profiles
SET status = sqlc.arg(status),
    reviewed_at = now(),
    updated_at = now()
WHERE profile_id = sqlc.arg(profile_id)
  AND status = 'pending'
RETURNING *;
