-- name: GetPublicCoachingPlan :one
-- 申し込む画面に出すプランとコーチ。公開中で、承認済みで、いまの版の誓約をしたコーチのものだけ。
SELECT
    pl.*,
    p.nickname AS coach_nickname,
    c.photo_public_id AS coach_photo_public_id,
    c.tier AS coach_tier,
    c.events AS coach_events,
    (SELECT count(*) FROM coaching_contracts ct WHERE ct.plan_id = pl.id AND ct.status IN ('accepted', 'active', 'suspended'))::int AS active_count
FROM coaching_plans pl
JOIN coach_profiles c ON c.profile_id = pl.coach_id
JOIN profiles p ON p.id = pl.coach_id
WHERE pl.id = sqlc.arg(id)
  AND pl.published
  AND c.status = 'approved'
  AND c.pledge_version = sqlc.arg(pledge_version)::smallint;

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
 WHERE ct.coach_id = sqlc.arg(coach_id) AND ct.status IN ('applied', 'accepted', 'active', 'suspended'))
UNION ALL
(SELECT ct.*, p.nickname AS requester_nickname
 FROM coaching_contracts ct
 JOIN profiles p ON p.id = ct.requester_id
 WHERE ct.coach_id = sqlc.arg(coach_id) AND ct.status NOT IN ('applied', 'accepted', 'active', 'suspended')
 ORDER BY ct.updated_at DESC
 LIMIT 30)
ORDER BY created_at DESC;

-- name: LockContractPlan :exec
-- 承諾で定員を数える前に、その申し込みのプランの行をロックする。
-- 同じプランの承諾を同時に送っても、1件ずつ数えるようにするため。
SELECT pl.id FROM coaching_plans pl
JOIN coaching_contracts ct ON ct.plan_id = pl.id
WHERE ct.id = $1
FOR UPDATE OF pl;

-- name: AcceptCoachingContract :one
-- 返事待ちで期限内のものだけ承諾できる。
-- 支払いがあるとき（payment_required）は、72時間以内に支払いが済んだら始まる（18歳未満は保護者が同意して払う）。
-- デモ版（支払いなし）は、大人は承諾したらそのまま期間が始まり、18歳未満は保護者が72時間以内に同意したら始まる。
-- 同じプランで枠を使っている人数が上限に届いていたら承諾できない（行が返らない）。
UPDATE coaching_contracts ct
SET status = CASE WHEN ct.minor OR sqlc.arg(payment_required)::boolean THEN 'accepted' ELSE 'active' END,
    payment_required = sqlc.arg(payment_required)::boolean,
    responded_at = now(),
    start_by = CASE WHEN ct.minor OR sqlc.arg(payment_required)::boolean THEN now() + interval '72 hours' END,
    starts_at = CASE WHEN ct.minor OR sqlc.arg(payment_required)::boolean THEN NULL ELSE now() END,
    ends_at = CASE WHEN ct.minor OR sqlc.arg(payment_required)::boolean THEN NULL ELSE now() + make_interval(days => ct.duration_days) END,
    updated_at = now()
FROM coaching_plans pl
WHERE ct.id = sqlc.arg(id)
  AND ct.coach_id = sqlc.arg(coach_id)
  AND ct.status = 'applied'
  AND ct.respond_by > now()
  AND pl.id = ct.plan_id
  AND (SELECT count(*) FROM coaching_contracts a WHERE a.plan_id = ct.plan_id AND a.status IN ('accepted', 'active', 'suspended')) < pl.capacity
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

-- name: ExpireCoachingStarts :many
-- 承諾から72時間のうちに保護者の同意（支払いがあるときは支払い）がなかったものを閉じる。
UPDATE coaching_contracts
SET status = 'expired',
    updated_at = now()
WHERE status = 'accepted' AND start_by <= now()
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

-- name: ListGuardianContracts :many
-- 保護者のページに出す、子の申し込み（申し込んだとき18歳未満だったもの）。
SELECT ct.*, p.nickname AS coach_nickname, c.photo_public_id AS coach_photo_public_id
FROM coaching_contracts ct
JOIN profiles p ON p.id = ct.coach_id
LEFT JOIN coach_profiles c ON c.profile_id = ct.coach_id
WHERE ct.requester_id = $1 AND ct.minor
ORDER BY ct.created_at DESC
LIMIT 50;

-- name: GuardianStartCoachingContract :one
-- デモ版（支払いなし）では、保護者がこの契約に同意したら、その時から期間が始まる。
UPDATE coaching_contracts
SET status = 'active',
    starts_at = now(),
    ends_at = now() + make_interval(days => duration_days),
    start_by = NULL,
    updated_at = now()
WHERE id = $1 AND requester_id = $2 AND minor AND NOT payment_required AND status = 'accepted' AND start_by > now()
RETURNING *;

-- name: GuardianDeclineCoachingContract :one
UPDATE coaching_contracts
SET status = 'guardian_declined',
    start_by = NULL,
    updated_at = now()
WHERE id = $1 AND requester_id = $2 AND minor AND status = 'accepted'
RETURNING *;

-- name: SuspendCoachingContract :one
-- 通報などで運営がやりとりを止める。期間は止めている間も進まないよう、再開のときに延ばす。
UPDATE coaching_contracts
SET status = 'suspended',
    suspended_at = now(),
    updated_at = now()
WHERE id = $1 AND status = 'active'
RETURNING *;

-- name: ResumeCoachingContract :one
-- 止めていた時間の分だけ終了日を延ばして再開する。
UPDATE coaching_contracts
SET status = 'active',
    ends_at = ends_at + (now() - suspended_at),
    suspended_at = NULL,
    updated_at = now()
WHERE id = $1 AND status = 'suspended'
RETURNING *;

-- name: AdminListCoachingContracts :many
-- 運営の契約一覧。会話の中身は含めない。通報の数と、連絡先の検出で止めた回数だけ数える。
SELECT
    ct.id, ct.plan_title, ct.plan_kind, ct.duration_days, ct.price_yen, ct.minor, ct.status,
    ct.starts_at, ct.ends_at, ct.created_at, ct.payment_required,
    COALESCE((SELECT pay.status FROM coaching_payments pay WHERE pay.contract_id = ct.id AND pay.status <> 'pending'
     ORDER BY pay.refund_reason IS NOT DISTINCT FROM 'not_started', pay.paid_at DESC LIMIT 1), '')::text AS payment_status,
    COALESCE((SELECT sum(pay.refunded_yen) FROM coaching_payments pay WHERE pay.contract_id = ct.id), 0)::int AS refunded_yen,
    rp.nickname AS requester_nickname,
    cp.nickname AS coach_nickname,
    (SELECT count(*) FROM reports r JOIN coaching_messages m ON m.id = r.target_id
     WHERE r.target_type = 'coaching_message' AND m.contract_id = ct.id)::int AS report_count,
    COALESCE((SELECT b.count FROM coaching_blocked_counts b WHERE b.contract_id = ct.id AND b.profile_id = ct.requester_id), 0)::int AS requester_blocked,
    COALESCE((SELECT b.count FROM coaching_blocked_counts b WHERE b.contract_id = ct.id AND b.profile_id = ct.coach_id), 0)::int AS coach_blocked
FROM coaching_contracts ct
JOIN profiles rp ON rp.id = ct.requester_id
JOIN profiles cp ON cp.id = ct.coach_id
ORDER BY ct.created_at DESC
LIMIT 100;

-- name: ListCoachingBlockedAccounts :many
-- 連絡先の検出で止めた回数が多いアカウント。
SELECT b.profile_id, p.nickname, sum(b.count)::int AS blocked, max(b.last_blocked_at)::timestamptz AS last_blocked_at
FROM coaching_blocked_counts b
JOIN profiles p ON p.id = b.profile_id
GROUP BY b.profile_id, p.nickname
ORDER BY blocked DESC, last_blocked_at DESC
LIMIT 50;

-- name: CoachingReportOnContract :one
-- その通報が、この契約のやりとり欄の1通に対するものか。
SELECT EXISTS (
    SELECT 1 FROM reports r
    JOIN coaching_messages m ON m.id = r.target_id
    WHERE r.id = sqlc.arg(report_id) AND r.target_type = 'coaching_message' AND m.contract_id = sqlc.arg(contract_id)
)::boolean;

-- name: InsertCoachingAccessLog :exec
INSERT INTO coaching_access_logs (id, admin_id, contract_id, report_id)
VALUES ($1, $2, $3, $4);

-- name: ListCoachingAccessLogs :many
SELECT l.id, l.admin_id, l.report_id, l.opened_at, p.nickname AS admin_nickname
FROM coaching_access_logs l
LEFT JOIN profiles p ON p.id = l.admin_id
WHERE l.contract_id = $1
ORDER BY l.opened_at DESC
LIMIT 100;

-- name: ListAdminProfileIDs :many
SELECT id FROM profiles WHERE is_admin;
