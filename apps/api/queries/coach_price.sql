-- コーチの価格申請制（本体 #1052）。

-- name: ListCoachPrices :many
-- いま有効な価格（行がないプランは未設定）。
SELECT coach_id, plan, price_yen, effective_at FROM coach_prices
WHERE coach_id = ANY(sqlc.arg(coach_ids)::uuid[]);

-- name: ListCoachPriceApplicationsFor :many
-- コーチ本人に見せる、承認待ちと切り替え待ちの申請。
SELECT * FROM coach_price_applications
WHERE coach_id = sqlc.arg(coach_id) AND status IN ('pending', 'approved')
ORDER BY plan, created_at;

-- name: UpsertPendingCoachPrice :one
-- 承認待ちがあれば金額を書き換え（上書き申請）、なければ新しく作る。
INSERT INTO coach_price_applications (id, coach_id, plan, price_yen, source)
VALUES (sqlc.arg(id), sqlc.arg(coach_id), sqlc.arg(plan), sqlc.arg(price_yen), sqlc.arg(source))
ON CONFLICT (coach_id, plan) WHERE status = 'pending'
DO UPDATE SET price_yen = EXCLUDED.price_yen, source = EXCLUDED.source, updated_at = now()
RETURNING *;

-- name: WithdrawPendingCoachPrice :one
UPDATE coach_price_applications
SET status = 'withdrawn', withdrawn_at = now(), updated_at = now()
WHERE coach_id = sqlc.arg(coach_id) AND plan = sqlc.arg(plan) AND status = 'pending'
RETURNING *;

-- name: ListAdminCoachPriceApplications :many
-- 運営の一覧。承認待ちを古い順に先に、そのあと承認済み・切り替え済みを新しい順に。
SELECT a.*, cp.name AS coach_name, cp.status AS coach_status, cp.country_code AS coach_country_code,
       approver.nickname AS approved_by_nickname
FROM coach_price_applications a
JOIN coach_profiles cp ON cp.profile_id = a.coach_id
LEFT JOIN profiles approver ON approver.id = a.approved_by
WHERE a.status IN ('pending', 'approved', 'applied')
ORDER BY (a.status = 'pending') DESC,
         CASE WHEN a.status = 'pending' THEN a.created_at END ASC,
         COALESCE(a.approved_at, a.created_at) DESC
LIMIT 200;

-- name: LockCoachPrices :exec
-- 同じコーチの承認・切り替えを1本ずつにする。
SELECT pg_advisory_xact_lock(hashtextextended('coach_prices:' || sqlc.arg(coach_id)::uuid::text, 0));

-- name: SupersedeApprovedCoachPrice :exec
UPDATE coach_price_applications
SET status = 'superseded', updated_at = now()
WHERE coach_id = sqlc.arg(coach_id) AND plan = sqlc.arg(plan) AND status = 'approved';

-- name: ApproveCoachPrice :one
UPDATE coach_price_applications
SET status = 'approved', approved_at = now(), approved_by = sqlc.arg(approved_by), updated_at = now()
WHERE id = sqlc.arg(id) AND status = 'pending'
RETURNING *;

-- name: GetCoachPriceApplication :one
SELECT * FROM coach_price_applications WHERE id = $1;

-- name: ApplyDueCoachPrices :many
-- 承認済みの申請のうち、承認の時点でそのコーチが対応中だった依頼（承認日時以前に作られ、まだ終わっていない依頼）が
-- 残っていないものを有効にする。返却・追加質問待ち（clarifying）・期限切れ・取り消し（削除）は終わったものとして数える。
-- coach_ids が空なら全コーチを対象にする（tick）。
WITH due AS (
    SELECT a.id FROM coach_price_applications a
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
    UPDATE coach_price_applications a
    SET status = 'applied', applied_at = now(), updated_at = now()
    FROM due WHERE a.id = due.id AND a.status = 'approved'
    RETURNING a.*
), upserted AS (
    INSERT INTO coach_prices (coach_id, plan, price_yen, application_id, effective_at)
    SELECT coach_id, plan, price_yen, id, applied_at FROM applied
    ON CONFLICT (coach_id, plan) DO UPDATE
    SET price_yen = EXCLUDED.price_yen, application_id = EXCLUDED.application_id, effective_at = EXCLUDED.effective_at
    RETURNING coach_id
)
SELECT applied.id, applied.coach_id, applied.plan, applied.price_yen, applied.applied_at FROM applied;

-- name: ListCoachApplyPrices :many
-- コーチ応募の審査画面に出す希望価格（応募フォームから出した申請の最新）。
SELECT DISTINCT ON (coach_id, plan) coach_id, plan, price_yen, status
FROM coach_price_applications
WHERE coach_id = ANY(sqlc.arg(coach_ids)::uuid[]) AND source = 'coach_apply' AND status <> 'withdrawn'
ORDER BY coach_id, plan, created_at DESC;
