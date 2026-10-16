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
SELECT count(*) FROM review_requests
WHERE requester_id = $1;

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

-- name: LockReviewAssignment :exec
-- 割り当ては1つずつ行い、同じコーチに上限を超えて回さないようにする。
SELECT pg_advisory_xact_lock(7351002);

-- name: ListQueuedReviewRequests :many
SELECT id FROM review_requests
WHERE status = 'queued'
  AND due_at > now()
ORDER BY created_at
LIMIT 50;

-- name: LockQueuedReviewRequest :one
SELECT * FROM review_requests
WHERE id = $1
  AND status = 'queued'
  AND due_at > now()
FOR UPDATE;

-- name: PickCoachForRequest :one
-- 枠と種目が合う受付中のコーチのうち、最後に打診された時刻が一番古い人。
-- 同じ依頼をまだ打診しておらず、持っている依頼と今日の本数が上限に届いていない人だけ。
SELECT c.profile_id
FROM coach_profiles c
WHERE c.status = 'approved'
  AND c.on_duty
  AND c.tier = sqlc.arg(tier)
  AND sqlc.arg(event)::text = ANY (c.events)
  AND c.profile_id <> sqlc.arg(requester_id)
  AND NOT EXISTS (
    SELECT 1 FROM review_offers o
    WHERE o.request_id = sqlc.arg(request_id)
      AND o.coach_id = c.profile_id
  )
  AND (
    SELECT count(*) FROM review_requests r
    WHERE r.coach_id = c.profile_id
      AND r.status IN ('offered', 'in_review')
  ) < sqlc.arg(max_active)::int
  AND (
    SELECT count(*) FROM review_offers o
    WHERE o.coach_id = c.profile_id
      AND (o.result IS NULL OR (o.result = 'accepted' AND o.answered_at >= sqlc.arg(day_start)::timestamptz))
  ) < c.daily_cap
ORDER BY c.last_offered_at NULLS FIRST, c.applied_at
LIMIT 1;

-- name: CreateReviewOffer :one
INSERT INTO review_offers (id, request_id, coach_id, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: MarkReviewRequestOffered :exec
UPDATE review_requests
SET status = 'offered',
    coach_id = $2,
    updated_at = now()
WHERE id = $1
  AND status = 'queued';

-- name: TouchCoachOffered :exec
UPDATE coach_profiles
SET last_offered_at = now()
WHERE profile_id = $1;

-- name: LockOpenReviewOffer :one
SELECT * FROM review_offers
WHERE id = $1
  AND coach_id = $2
  AND result IS NULL
FOR UPDATE;

-- name: AnswerReviewOffer :exec
UPDATE review_offers
SET result = sqlc.arg(result)::text,
    answered_at = now()
WHERE id = sqlc.arg(id)
  AND result IS NULL;

-- name: AcceptReviewRequest :one
UPDATE review_requests
SET status = 'in_review',
    updated_at = now()
WHERE id = $1
  AND coach_id = $2
  AND status = 'offered'
  AND due_at > now()
RETURNING *;

-- name: RequeueReviewRequest :exec
UPDATE review_requests
SET status = 'queued',
    coach_id = NULL,
    updated_at = now()
WHERE id = $1
  AND status = 'offered';

-- name: ExpireStaleReviewOffers :many
-- 30分のうちに返事がなかった打診を閉じ、依頼を待ちに戻す。
WITH stale AS (
    UPDATE review_offers
    SET result = 'expired',
        answered_at = now()
    WHERE result IS NULL
      AND expires_at <= now()
    RETURNING request_id
)
UPDATE review_requests r
SET status = 'queued',
    coach_id = NULL,
    updated_at = now()
FROM stale
WHERE r.id = stale.request_id
  AND r.status = 'offered'
RETURNING r.id;

-- name: ExpireOverdueReviewRequests :many
-- 返却の目安を過ぎた依頼を expired にする。添削中だった依頼はコーチを残し、遵守率に使う。
WITH target AS (
    SELECT id, status AS old_status
    FROM review_requests
    WHERE status IN ('queued', 'offered', 'in_review')
      AND due_at <= now()
    FOR UPDATE
), overdue AS (
    UPDATE review_requests r
    SET status = 'expired',
        coach_id = CASE WHEN target.old_status = 'in_review' THEN r.coach_id END,
        updated_at = now()
    FROM target
    WHERE r.id = target.id
    RETURNING r.id, r.requester_id, r.coach_id, target.old_status
), closed AS (
    UPDATE review_offers o
    SET result = 'expired',
        answered_at = now()
    FROM overdue
    WHERE o.request_id = overdue.id
      AND o.result IS NULL
)
SELECT id, requester_id, coach_id, old_status FROM overdue;

-- name: CountOnDutyCoaches :one
SELECT count(*) FROM coach_profiles
WHERE status = 'approved'
  AND on_duty
  AND tier = sqlc.arg(tier)
  AND sqlc.arg(event)::text = ANY (events)
  AND profile_id <> sqlc.arg(requester_id);

-- name: CountWaitingReviewRequests :one
SELECT count(*) FROM review_requests
WHERE status IN ('queued', 'offered')
  AND tier = $1
  AND event = $2;

-- name: ListCoachOpenOffers :many
SELECT o.id AS offer_id, o.expires_at AS offer_expires_at, sqlc.embed(r)
FROM review_offers o
JOIN review_requests r ON r.id = o.request_id
WHERE o.coach_id = $1
  AND o.result IS NULL
  AND o.expires_at > now()
ORDER BY o.offered_at;

-- name: ListCoachReviewing :many
SELECT * FROM review_requests
WHERE coach_id = sqlc.arg(coach_id)::uuid
  AND status = 'in_review'
ORDER BY due_at;
