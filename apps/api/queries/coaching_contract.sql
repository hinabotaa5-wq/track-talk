-- name: GetPublicCoachingPlan :one
-- 申し込む画面に出すプランとコーチ。公開中で、承認済みで、いまの版の誓約をしたコーチのものだけ。
SELECT
    pl.*,
    p.nickname AS coach_nickname,
    c.photo_public_id AS coach_photo_public_id,
    c.tier AS coach_tier,
    c.events AS coach_events,
    (SELECT count(*) FROM coaching_contracts ct WHERE ct.plan_id = pl.id AND ct.status = 'active')::int AS active_count
FROM coaching_plans pl
JOIN coach_profiles c ON c.profile_id = pl.coach_id
JOIN profiles p ON p.id = pl.coach_id
WHERE pl.id = sqlc.arg(id)
  AND pl.published
  AND c.status = 'approved'
  AND c.pledge_version = sqlc.arg(pledge_version)::smallint;

-- name: CountActiveContractsByPlan :one
SELECT count(*)::int FROM coaching_contracts
WHERE plan_id = $1 AND status = 'active';

-- name: CreateCoachingContract :one
INSERT INTO coaching_contracts (
    id, plan_id, coach_id, requester_id, plan_kind, plan_title, duration_days, price_yen,
    event, goal, current_record, message, minor, respond_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING *;

-- name: InsertContractConsent :exec
INSERT INTO coaching_contract_consents (contract_id, party, profile_id, version, report_review, contact_detection)
VALUES ($1, $2, $3, $4, true, true);

-- name: GetCoachingContract :one
SELECT * FROM coaching_contracts
WHERE id = $1;

-- name: ListMyCoachingContracts :many
-- 利用者が申し込んだもの。コーチのニックネームと顔写真をつける。
SELECT ct.*, p.nickname AS coach_nickname, c.photo_public_id AS coach_photo_public_id
FROM coaching_contracts ct
JOIN profiles p ON p.id = ct.coach_id
LEFT JOIN coach_profiles c ON c.profile_id = ct.coach_id
WHERE ct.requester_id = $1
ORDER BY ct.created_at DESC
LIMIT 100;

-- name: ListCoachContracts :many
-- コーチに届いた申し込み。利用者はニックネームだけ見せる。終わったものは新しい順に30件まで。
(SELECT ct.*, p.nickname AS requester_nickname
 FROM coaching_contracts ct
 JOIN profiles p ON p.id = ct.requester_id
 WHERE ct.coach_id = sqlc.arg(coach_id) AND ct.status IN ('applied', 'active'))
UNION ALL
(SELECT ct.*, p.nickname AS requester_nickname
 FROM coaching_contracts ct
 JOIN profiles p ON p.id = ct.requester_id
 WHERE ct.coach_id = sqlc.arg(coach_id) AND ct.status NOT IN ('applied', 'active')
 ORDER BY ct.updated_at DESC
 LIMIT 30)
ORDER BY created_at DESC;

-- name: AcceptCoachingContract :one
-- 返事待ちで期限内のものだけ承諾できる。デモ版なので、承諾したらそのまま期間が始まる。
-- 同じプランで期間中の人数が上限に届いていたら承諾できない（行が返らない）。
UPDATE coaching_contracts ct
SET status = 'active',
    responded_at = now(),
    starts_at = now(),
    ends_at = now() + make_interval(days => ct.duration_days),
    updated_at = now()
FROM coaching_plans pl
WHERE ct.id = sqlc.arg(id)
  AND ct.coach_id = sqlc.arg(coach_id)
  AND ct.status = 'applied'
  AND ct.respond_by > now()
  AND pl.id = ct.plan_id
  AND (SELECT count(*) FROM coaching_contracts a WHERE a.plan_id = ct.plan_id AND a.status = 'active') < pl.capacity
RETURNING ct.*;

-- name: DeclineCoachingContract :one
UPDATE coaching_contracts
SET status = 'declined',
    responded_at = now(),
    updated_at = now()
WHERE id = $1 AND coach_id = $2 AND status = 'applied'
RETURNING *;

-- name: CancelCoachingContract :one
-- 利用者が、コーチの返事の前に取り下げる。
UPDATE coaching_contracts
SET status = 'cancelled',
    updated_at = now()
WHERE id = $1 AND requester_id = $2 AND status = 'applied'
RETURNING *;

-- name: ExpireCoachingApplications :many
-- 48時間返事がなかった申し込みを閉じる。
UPDATE coaching_contracts
SET status = 'expired',
    updated_at = now()
WHERE status = 'applied' AND respond_by <= now()
RETURNING *;

-- name: EndCoachingContracts :many
-- 期間が終わったものを閉じる。
UPDATE coaching_contracts
SET status = 'ended',
    updated_at = now()
WHERE status = 'active' AND ends_at <= now()
RETURNING *;

-- name: GetCoachingContractDetail :one
-- 申し込みの中身と、両方のニックネーム、コーチの顔写真。
SELECT ct.*,
    cp.nickname AS coach_nickname,
    c.photo_public_id AS coach_photo_public_id,
    rp.nickname AS requester_nickname
FROM coaching_contracts ct
JOIN profiles cp ON cp.id = ct.coach_id
JOIN profiles rp ON rp.id = ct.requester_id
LEFT JOIN coach_profiles c ON c.profile_id = ct.coach_id
WHERE ct.id = $1;
