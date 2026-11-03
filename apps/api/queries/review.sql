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
-- 送った依頼の数。支払いがまだのものも数える（数えないと、保護者の同意前に何本でも作ってから払える）。
-- 払わないまま閉じたものは tick が消すので、そのあとは数に入らない。
SELECT count(*) FROM review_requests
WHERE requester_id = $1;

-- name: CreateReviewRequest :one
-- status は queued（支払いなしのデモ版）か awaiting_payment（支払いが届いたら queued になる）。
INSERT INTO review_requests (id, requester_id, tier, event, focus, video_public_id, due_at, status, question, requested_coach_id, requested_coach_name, base_price_yen, use_tickets)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
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
  AND (sqlc.narg(requested_coach_id)::uuid IS NULL OR c.profile_id = sqlc.narg(requested_coach_id)::uuid)
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
      AND r.status IN ('offered', 'in_review', 'needs_info')
  ) < sqlc.arg(max_active)::int
  AND (
    SELECT count(*) FROM review_offers o
    WHERE o.coach_id = c.profile_id
      AND (o.result IS NULL OR (o.result = 'accepted' AND o.answered_at >= sqlc.arg(day_start)::timestamptz))
  ) < c.daily_cap
  AND NOT EXISTS (
    -- 評価が低いコーチは順番から外す（評価が min_ratings 件以上で、平均の★が low_stars 未満）。
    SELECT 1 FROM review_ratings rt
    JOIN review_responses rs ON rs.request_id = rt.request_id
    WHERE rs.coach_id = c.profile_id
      AND rt.rated_at > coalesce(c.rating_reset_at, '-infinity')
    HAVING count(rt.stars) >= sqlc.arg(min_ratings)::int
       AND avg(rt.stars)::float8 < sqlc.arg(low_stars)::float8
  )
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
SET status = CASE WHEN requested_coach_id IS NULL THEN 'queued' ELSE 'expired' END,
    coach_id = NULL,
    updated_at = now()
WHERE id = $1
  AND status = 'offered';

-- name: ExpireStaleReviewOffers :many
-- 引受期限（90分、夜間は数えない）までに返事がなかった打診を閉じ、依頼を待ちに戻す。
WITH stale AS (
    UPDATE review_offers
    SET result = 'expired',
        answered_at = now()
    WHERE result IS NULL
      AND expires_at <= now()
    RETURNING request_id
)
UPDATE review_requests r
SET status = CASE WHEN requested_coach_id IS NULL THEN 'queued' ELSE 'expired' END,
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
SELECT count(*) FROM coach_profiles c
WHERE c.status = 'approved'
  AND c.on_duty
  AND c.tier = sqlc.arg(tier)
  AND sqlc.arg(event)::text = ANY (c.events)
  AND c.profile_id <> sqlc.arg(requester_id)
  AND NOT EXISTS (
    -- 評価が低いコーチは順番から外す（評価が min_ratings 件以上で、平均の★が low_stars 未満）。
    SELECT 1 FROM review_ratings rt
    JOIN review_responses rs ON rs.request_id = rt.request_id
    WHERE rs.coach_id = c.profile_id
      AND rt.rated_at > coalesce(c.rating_reset_at, '-infinity')
    HAVING count(rt.stars) >= sqlc.arg(min_ratings)::int
       AND avg(rt.stars)::float8 < sqlc.arg(low_stars)::float8
  );

-- name: ListCoachOpenOffers :many
SELECT o.id AS offer_id, o.expires_at AS offer_expires_at, sqlc.embed(r)
FROM review_offers o
JOIN review_requests r ON r.id = o.request_id
WHERE o.coach_id = $1
  AND o.result IS NULL
  AND o.expires_at > now()
  -- 差し戻し中（needs_info）の打診は、再提出されるまで出さない。
  AND r.status = 'offered'
ORDER BY o.offered_at;

-- name: ListCoachReviewing :many
SELECT * FROM review_requests
WHERE coach_id = sqlc.arg(coach_id)::uuid
  AND (status IN ('in_review', 'clarifying')
       -- 差し戻し中の依頼も、再提出を待っていることが分かるように出す。
       OR status = 'needs_info')
ORDER BY due_at;

-- name: LockReviewRequester :exec
-- 同じ人の依頼の作成を1つずつにする。「同意待ちの18歳未満は1本目まで」の確認と作成のあいだに、別の作成を割り込ませない。
SELECT pg_advisory_xact_lock(hashtext('review_requester:' || sqlc.arg(requester_id)::uuid::text));

-- name: ListReviewNominees :many
-- 動画添削で選べるコーチ。受付中で、担当種目・受付上限・報酬口座の条件を満たす人。
-- 並びは人気や実績によらず日替わり(コーチの ID と今日の日付から決める)にし、全員が上に来る日が同じように回る。
SELECT c.profile_id, COALESCE(NULLIF(c.name, ''),p.nickname, 'コーチ')::text AS nickname, c.events, c.years, c.best_record, c.achievements, c.photo_public_id
FROM coach_profiles c JOIN profiles p ON p.id = c.profile_id
WHERE c.status = 'approved'
  AND c.on_duty
  AND (NOT sqlc.arg(payments)::boolean OR c.payouts_enabled)
  AND c.tier = sqlc.arg(tier)
  AND (sqlc.narg(requested_coach_id)::uuid IS NULL OR c.profile_id = sqlc.narg(requested_coach_id)::uuid)
  AND sqlc.arg(event)::text = ANY (c.events)
  AND c.profile_id <> sqlc.arg(requester_id)
  AND (
    SELECT count(*) FROM review_requests r
    WHERE r.coach_id = c.profile_id
      AND r.status IN ('offered', 'in_review', 'needs_info')
  ) < sqlc.arg(max_active)::int
  AND (
    SELECT count(*) FROM review_offers o
    WHERE o.coach_id = c.profile_id
      AND (o.result IS NULL OR (o.result = 'accepted' AND o.answered_at >= sqlc.arg(day_start)::timestamptz))
  ) < c.daily_cap
  AND NOT EXISTS (
    -- 評価が低いコーチは順番から外す（評価が min_ratings 件以上で、平均の★が low_stars 未満）。
    SELECT 1 FROM review_ratings rt
    JOIN review_responses rs ON rs.request_id = rt.request_id
    WHERE rs.coach_id = c.profile_id
      AND rt.rated_at > coalesce(c.rating_reset_at, '-infinity')
    HAVING count(rt.stars) >= sqlc.arg(min_ratings)::int
       AND avg(rt.stars)::float8 < sqlc.arg(low_stars)::float8
  )
ORDER BY md5(c.profile_id::text || extract(epoch FROM sqlc.arg(day_start)::timestamptz)::bigint::text)
LIMIT 200;

-- name: RequestReviewInfo :one
-- 担当のコーチが、内容確認のために依頼を1回だけ差し戻す。引受前（offered）でも引受後（in_review）でもよく、返却期限までに限る。
-- 差し戻す前の状態を info_from_status に残す（SET の右辺の status は更新前の値）。
UPDATE review_requests
SET status = 'needs_info',
    info_request = sqlc.arg(body)::text,
    info_reason = sqlc.narg(reason)::text,
    info_requested_at = now(),
    info_from_status = status,
    resubmit_by = sqlc.arg(resubmit_by)::timestamptz,
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND coach_id = sqlc.arg(coach_id)::uuid
  AND status IN ('offered', 'in_review')
  AND info_requested_at IS NULL
  AND due_at > now()
RETURNING *;

-- name: LockCoachOpenOfferForRequest :one
-- 差し戻す依頼の、まだ返事をしていない打診（引受前の差し戻しのとき）。
SELECT * FROM review_offers
WHERE request_id = $1
  AND coach_id = $2
  AND result IS NULL
FOR UPDATE;

-- name: SetReviewOfferExpiry :execrows
-- 打診の引受期限を置き直す。差し戻し中は再提出期限まで延ばしておき、再提出で引受期限90分を数え直す。
UPDATE review_offers
SET expires_at = sqlc.arg(expires_at)::timestamptz
WHERE request_id = sqlc.arg(request_id)
  AND coach_id = sqlc.arg(coach_id)
  AND result IS NULL;

-- name: ResubmitReviewRequest :one
-- 依頼者が、差し戻された依頼の相談内容を直して（動画は差し替えてもよい）再提出する。再提出期限までに1回だけ。
-- 返却期限は再提出から24時間に数え直し、差し戻す前の状態（引受前・引受後）に戻す。
UPDATE review_requests
SET status = info_from_status,
    original_question = question,
    question = sqlc.arg(question)::text,
    video_public_id = sqlc.arg(video_public_id)::text,
    video_replaced = (sqlc.arg(video_public_id)::text <> video_public_id),
    due_at = sqlc.arg(due_at)::timestamptz,
    resubmitted_at = now(),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND requester_id = sqlc.arg(requester_id)
  AND status = 'needs_info'
  AND resubmit_by > now()
RETURNING *;

-- name: ExpireUnresubmittedReviewRequests :many
-- 再提出期限までに再提出がなかった差し戻しを終える。既存の期限切れと同じく expired にし、
-- 支払いの取り消し・返金とチケットの返却は tick の既存の処理が行う。返事待ちのままの打診も閉じる。
WITH target AS (
    SELECT id, info_from_status
    FROM review_requests
    WHERE status = 'needs_info'
      AND resubmit_by <= now()
    FOR UPDATE
), ended AS (
    UPDATE review_requests r
    SET status = 'expired',
        updated_at = now()
    FROM target
    WHERE r.id = target.id
    RETURNING r.id, r.requester_id, r.coach_id, target.info_from_status
), closed AS (
    UPDATE review_offers o
    SET result = 'expired',
        answered_at = now()
    FROM ended
    WHERE o.request_id = ended.id
      AND o.result IS NULL
)
SELECT id, requester_id, coach_id, info_from_status FROM ended;
