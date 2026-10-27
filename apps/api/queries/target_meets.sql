-- name: GetTargetMeet :one
SELECT source, ref, name, starts_on, ends_on FROM target_meets WHERE profile_id = $1;

-- name: SetTargetMeet :one
INSERT INTO target_meets (profile_id, source, ref, name, starts_on, ends_on)
VALUES (sqlc.arg(profile_id), sqlc.arg(source), sqlc.arg(ref), sqlc.arg(name), sqlc.arg(starts_on), sqlc.narg(ends_on))
ON CONFLICT (profile_id) DO UPDATE
SET source = EXCLUDED.source,
    ref = EXCLUDED.ref,
    name = EXCLUDED.name,
    starts_on = EXCLUDED.starts_on,
    ends_on = EXCLUDED.ends_on,
    updated_at = now()
RETURNING source, ref, name, starts_on, ends_on;

-- name: DeleteTargetMeet :exec
DELETE FROM target_meets WHERE profile_id = $1;

-- name: GetDomesticMajorForTarget :one
SELECT name, starts_on, ends_on FROM domestic_majors WHERE slug = $1 AND year = $2;

-- name: GetCompetitionForTarget :one
SELECT name, starts_on, ends_on FROM competitions WHERE id = $1;
