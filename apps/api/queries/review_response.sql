-- name: CreateReviewResponse :one
INSERT INTO review_responses (request_id, coach_id, good_point, fix_points, drill, demo_video_public_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: MarkReviewReturned :one
-- 返答できるのは、引き受けたコーチが返却目安までに出すときだけ。
UPDATE review_requests
SET status = 'returned',
    returned_at = now(),
    updated_at = now()
WHERE id = $1
  AND coach_id = $2
  AND status = 'in_review'
  AND due_at > now()
RETURNING *;

-- name: GetReviewResponse :one
SELECT * FROM review_responses
WHERE request_id = $1;

-- name: GetCoachReviewRequest :one
SELECT * FROM review_requests
WHERE id = sqlc.arg(id)
  AND coach_id = sqlc.arg(coach_id)::uuid
  AND status IN ('in_review', 'returned', 'clarifying');

-- name: RequestReviewClarification :one
-- 「わからなかった」は1つの返答に1回だけ。
UPDATE review_responses
SET clarify_body = sqlc.narg(body)::text,
    clarify_reason = sqlc.arg(reason)::text,
    clarify_requested_at = now()
WHERE request_id = sqlc.arg(request_id)
  AND clarify_requested_at IS NULL
  AND hidden_at IS NULL
RETURNING *;

-- name: MarkReviewClarifying :one
UPDATE review_requests
SET status = 'clarifying',
    updated_at = now()
WHERE id = $1
  AND requester_id = $2
  AND status = 'returned'
  AND returned_at > now()-interval '7 days'
RETURNING *;

-- name: SaveReviewClarification :one
UPDATE review_responses
SET clarification = sqlc.arg(clarification)::text,
    clarified_at = now()
WHERE request_id = sqlc.arg(request_id)
  AND clarify_requested_at IS NOT NULL
  AND clarification IS NULL
RETURNING *;

-- name: MarkReviewClarified :one
UPDATE review_requests
SET status = 'returned',
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND coach_id = sqlc.arg(coach_id)::uuid
  AND status = 'clarifying'
RETURNING *;

-- name: GetReviewRating :one
SELECT * FROM review_ratings
WHERE request_id = $1;

-- name: SaveReviewStars :one
INSERT INTO review_ratings (request_id, stars, rated_at)
VALUES (sqlc.arg(request_id), sqlc.arg(stars)::smallint, now())
ON CONFLICT (request_id) DO UPDATE
SET stars = EXCLUDED.stars,
    rated_at = now()
RETURNING *;

-- name: SaveReviewHelpful :one
INSERT INTO review_ratings (request_id, helpful, helpful_at)
VALUES (sqlc.arg(request_id), sqlc.arg(helpful)::text, now())
ON CONFLICT (request_id) DO UPDATE
SET helpful = EXCLUDED.helpful,
    helpful_at = now()
RETURNING *;

-- name: MarkHelpfulAsksDue :many
-- 返却から7日たった返答に「役に立ちましたか」を一度だけ聞く。
UPDATE review_responses rs
SET helpful_asked_at = now()
FROM review_requests r
WHERE r.id = rs.request_id
  AND r.status IN ('returned', 'clarifying')
  AND r.returned_at <= now() - interval '7 days'
  AND rs.helpful_asked_at IS NULL
  AND rs.hidden_at IS NULL
RETURNING r.id, r.requester_id;

-- name: ReviewResponseReportable :one
-- 返答を通報できるのは、その依頼を送った本人だけ。
SELECT EXISTS (
    SELECT 1
    FROM review_responses rs
    JOIN review_requests r ON r.id = rs.request_id
    WHERE rs.request_id = sqlc.arg(request_id)
      AND r.requester_id = sqlc.arg(requester_id)
)::boolean;

-- name: HideReviewResponse :execrows
UPDATE review_responses
SET hidden_at = COALESCE(hidden_at, now())
WHERE request_id = $1;

-- name: ListCoachProbation :many
-- 各コーチの最初の20本の返答。運営が読んで、問題がないかを確かめる（試用期間）。
SELECT
    rs.request_id,
    rs.coach_id,
    p.nickname AS coach_nickname,
    ranked.n::int AS nth,
    r.tier,
    r.event,
    rs.good_point,
    rs.fix_points,
    rs.drill,
    rs.clarification,
    rs.created_at,
    rs.hidden_at
FROM (
    SELECT request_id, row_number() OVER (PARTITION BY coach_id ORDER BY created_at) AS n
    FROM review_responses
    WHERE coach_id IS NOT NULL
) ranked
JOIN review_responses rs ON rs.request_id = ranked.request_id
JOIN review_requests r ON r.id = rs.request_id
LEFT JOIN profiles p ON p.id = rs.coach_id
WHERE ranked.n <= 20
ORDER BY rs.created_at DESC
LIMIT 100;

-- name: GetReviewCoach :one
-- 返答と一緒に利用者へ見せるコーチのプロフィール。
SELECT COALESCE(NULLIF(c.name, ''), p.nickname, '')::text AS nickname, c.photo_public_id, c.tier, c.events, c.best_record, c.achievements, c.years
FROM coach_profiles c
JOIN profiles p ON p.id = c.profile_id
WHERE c.profile_id = $1;

-- name: GetCoachRatingSummary :one
-- コーチが受けた★の件数と平均。運営が順番に戻したあとの★だけを数える。
SELECT count(rt.stars)::int AS count, coalesce(avg(rt.stars), 0)::float8 AS average
FROM review_ratings rt
JOIN review_responses rs ON rs.request_id = rt.request_id
JOIN coach_profiles c ON c.profile_id = rs.coach_id
WHERE rs.coach_id = sqlc.arg(coach_id)::uuid
  AND rt.rated_at > coalesce(c.rating_reset_at, '-infinity');
