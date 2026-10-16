-- name: UpdateCoachDuty :one
-- 承認済みのコーチだけが受付中にできる。
UPDATE coach_profiles
SET on_duty = sqlc.arg(on_duty),
    daily_cap = sqlc.arg(daily_cap),
    updated_at = now()
WHERE profile_id = sqlc.arg(profile_id)
  AND status = 'approved'
RETURNING *;

-- name: CreateDemoCode :one
INSERT INTO coaching_demo_codes (code)
VALUES ($1)
RETURNING *;

-- name: ListDemoCodes :many
SELECT c.code, c.created_at, c.used_at, p.nickname AS used_by_nickname
FROM coaching_demo_codes c
LEFT JOIN profiles p ON p.id = c.used_by
ORDER BY c.created_at DESC
LIMIT 100;

-- name: UseDemoCode :one
-- まだ使われていないコードだけを使える。
UPDATE coaching_demo_codes
SET used_by = $2,
    used_at = now()
WHERE code = $1
  AND used_at IS NULL
RETURNING *;

-- name: SetCoachingDemo :exec
UPDATE profiles
SET coaching_demo = true,
    updated_at = now()
WHERE id = $1;

-- name: CountReviewRequestsByRequester :one
-- 取り消したものは数えない。
SELECT count(*) FROM review_requests
WHERE requester_id = $1
  AND status <> 'canceled';

-- name: CreateReviewRequest :one
INSERT INTO review_requests (id, requester_id, tier, event, focus, video_public_id, due_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListMyReviewRequests :many
SELECT * FROM review_requests
WHERE requester_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountMyReviewRequests :one
SELECT count(*) FROM review_requests
WHERE requester_id = $1;

-- name: GetReviewRequest :one
SELECT * FROM review_requests
WHERE id = $1;

-- name: CancelReviewRequest :one
-- まだコーチに回っていないものだけ取り消せる。
UPDATE review_requests
SET status = 'canceled',
    canceled_at = now(),
    updated_at = now()
WHERE id = $1
  AND requester_id = $2
  AND status = 'queued'
RETURNING *;
