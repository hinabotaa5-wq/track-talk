-- name: ListCoachAvailability :many
SELECT * FROM coach_availability
WHERE coach_id = $1 AND day >= sqlc.arg(today)::date AND day < sqlc.arg(today)::date + 21
ORDER BY day;

-- name: HasPublicAvailabilityPlan :one
SELECT EXISTS (
    SELECT 1 FROM coaching_plans pl
    JOIN coach_profiles c ON c.profile_id = pl.coach_id
    WHERE pl.coach_id = sqlc.arg(coach_id) AND pl.published
      AND c.status = 'approved' AND c.pledge_version >= sqlc.arg(pledge_version)::smallint
      AND pl.service_type IN ('in_person', 'competition_support')
      AND pl.service_type = ANY(c.service_types)
      AND (pl.service_type <> 'competition_support' OR
          (pl.competition_date >= (now() AT TIME ZONE 'Asia/Tokyo')::date AND pl.support_start_date IS NOT NULL))
)::boolean;

-- name: UpsertCoachAvailability :exec
INSERT INTO coach_availability (coach_id, day, status, note)
VALUES ($1, $2, $3, $4)
ON CONFLICT (coach_id, day) DO UPDATE
SET status = EXCLUDED.status, note = EXCLUDED.note, updated_at = now();

-- name: DeleteCoachAvailabilityDay :exec
DELETE FROM coach_availability WHERE coach_id = $1 AND day = $2;

-- name: DeletePastCoachAvailability :exec
DELETE FROM coach_availability WHERE coach_id = $1 AND day < sqlc.arg(today)::date;
