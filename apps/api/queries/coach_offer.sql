-- コーチの添削項目（追加項目）と見本動画の変更申請（本体 #1086）。価格申請（coach_price.sql）と同じ流儀。

-- name: ListCoachOffers :many
-- いま有効な追加項目（行がないコーチは追加項目なし・見本動画なし）。
SELECT o.coach_id, o.application_id, o.effective_at, a.items
FROM coach_offers o
JOIN coach_offer_applications a ON a.id = o.application_id
WHERE o.coach_id = ANY(sqlc.arg(coach_ids)::uuid[]);

-- name: ListCoachOfferApplicationsFor :many
-- コーチ本人に見せる、承認待ちと切り替え待ちの申請。
SELECT * FROM coach_offer_applications
WHERE coach_id = sqlc.arg(coach_id) AND status IN ('pending', 'approved')
ORDER BY created_at;

-- name: ListCoachOfferApplicationVideos :many
-- 申請の見本動画（提出した順）。
SELECT * FROM coach_offer_application_videos
WHERE application_id = ANY(sqlc.arg(application_ids)::uuid[])
ORDER BY application_id, position;

-- name: UpsertPendingCoachOffer :one
-- 承認待ちがあればセットを書き換え（上書き申請）、なければ新しく作る。見本動画はこのあと入れ直す。
INSERT INTO coach_offer_applications (id, coach_id, items)
VALUES (sqlc.arg(id), sqlc.arg(coach_id), sqlc.arg(items)::text[])
ON CONFLICT (coach_id) WHERE status = 'pending'
DO UPDATE SET items = EXCLUDED.items, updated_at = now()
RETURNING *;

-- name: DeleteCoachOfferApplicationVideos :exec
DELETE FROM coach_offer_application_videos WHERE application_id = $1;

-- name: InsertCoachOfferApplicationVideo :exec
INSERT INTO coach_offer_application_videos (application_id, position, video_public_id)
VALUES ($1, $2, $3);

-- name: WithdrawPendingCoachOffer :one
UPDATE coach_offer_applications
SET status = 'withdrawn', withdrawn_at = now(), updated_at = now()
WHERE coach_id = sqlc.arg(coach_id) AND status = 'pending'
RETURNING *;

-- name: ListAdminCoachOfferApplications :many
-- 運営の一覧。承認待ちを古い順に先に、そのあと承認済み・切り替え済みを新しい順に。
SELECT a.*, cp.name AS coach_name, cp.status AS coach_status,
       approver.nickname AS approved_by_nickname
FROM coach_offer_applications a
JOIN coach_profiles cp ON cp.profile_id = a.coach_id
LEFT JOIN profiles approver ON approver.id = a.approved_by
WHERE a.status IN ('pending', 'approved', 'applied')
ORDER BY (a.status = 'pending') DESC,
         CASE WHEN a.status = 'pending' THEN a.created_at END ASC,
         COALESCE(a.approved_at, a.created_at) DESC
LIMIT 200;

-- name: LockCoachOffers :exec
-- 同じコーチの申請・承認・切り替えを1本ずつにする。
SELECT pg_advisory_xact_lock(hashtextextended('coach_offers:' || sqlc.arg(coach_id)::uuid::text, 0));

-- name: SupersedeApprovedCoachOffer :exec
UPDATE coach_offer_applications
SET status = 'superseded', updated_at = now()
WHERE coach_id = sqlc.arg(coach_id) AND status = 'approved';

-- name: ApproveCoachOffer :one
UPDATE coach_offer_applications
SET status = 'approved', approved_at = now(), approved_by = sqlc.arg(approved_by), updated_at = now()
WHERE id = sqlc.arg(id) AND status = 'pending'
RETURNING *;

-- name: GetCoachOfferApplication :one
SELECT * FROM coach_offer_applications WHERE id = $1;

-- name: ApplyDueCoachOffers :many
-- 承認済みの申請のうち、承認の時点でそのコーチが対応中だった依頼が残っていないものを有効にする。
-- 「対応中」の定義は価格の切り替え（ApplyDueCoachPrices、000194）と同じ。coach_ids が空なら全コーチ（tick）。
WITH due AS (
    SELECT a.id FROM coach_offer_applications a
    WHERE a.status = 'approved'
      AND (cardinality(sqlc.arg(coach_ids)::uuid[]) = 0 OR a.coach_id = ANY(sqlc.arg(coach_ids)::uuid[]))
      AND NOT EXISTS (
        SELECT 1 FROM review_requests r
        WHERE (r.requested_coach_id = a.coach_id OR r.coach_id = a.coach_id)
          AND r.created_at <= a.approved_at
          AND r.status IN ('awaiting_payment', 'queued', 'offered', 'in_review', 'needs_info')
      )
    FOR UPDATE OF a SKIP LOCKED
), applied AS (
    UPDATE coach_offer_applications a
    SET status = 'applied', applied_at = now(), updated_at = now()
    FROM due WHERE a.id = due.id AND a.status = 'approved'
    RETURNING a.*
), upserted AS (
    INSERT INTO coach_offers (coach_id, application_id, effective_at)
    SELECT coach_id, id, applied_at FROM applied
    ON CONFLICT (coach_id) DO UPDATE
    SET application_id = EXCLUDED.application_id, effective_at = EXCLUDED.effective_at
    RETURNING coach_id
)
SELECT applied.id, applied.coach_id, applied.applied_at FROM applied;

-- name: SetReviewRequestOfferItems :exec
-- 依頼の作成時に、指名したコーチの有効な追加項目のタイトルを固定する。
UPDATE review_requests SET offer_items = sqlc.arg(items)::text[] WHERE id = sqlc.arg(id);

-- name: CreateReviewResponseItem :exec
INSERT INTO review_response_items (request_id, position, title, body)
VALUES ($1, $2, $3, $4);

-- name: ListReviewResponseItems :many
SELECT * FROM review_response_items
WHERE request_id = ANY(sqlc.arg(request_ids)::uuid[])
ORDER BY request_id, position;
