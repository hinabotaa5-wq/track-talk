-- 大会までプラン（無料相談の提案から成立した、動画への返答が無制限の契約）の途中終了とコーチ報酬の精算。
-- 対象の契約は、fulfillment_version=1・video_reviews=0・service_type='competition_support'。

-- name: ListSupportAdvanceCandidates :many
-- 月次の仮払いを記録する候補。支払い済みで期間中（止めていない）の契約と、これまでの仮払いの合計。
SELECT ct.*, COALESCE((SELECT sum(a.amount_yen) FROM support_coach_advances a WHERE a.contract_id = ct.id), 0)::int AS advanced_yen
FROM coaching_contracts ct
WHERE ct.fulfillment_version = 1 AND ct.video_reviews = 0 AND ct.service_type = 'competition_support'
  AND ct.status = 'active' AND ct.fulfillment_state = 'open' AND NOT ct.support_coach_hold
  AND ct.starts_at IS NOT NULL AND ct.starts_at < sqlc.arg(through_end)::timestamptz
  AND EXISTS (SELECT 1 FROM coaching_payments pay WHERE pay.contract_id = ct.id AND pay.status = 'paid')
ORDER BY ct.starts_at, ct.id
LIMIT 200;

-- name: InsertSupportAdvanceEarning :one
INSERT INTO coach_earnings (id, coach_id, source, gross_yen, share_percent, amount_yen, confirmed_at)
VALUES (sqlc.arg(id), sqlc.arg(coach_id), 'support_advance', sqlc.arg(amount_yen), 100, sqlc.arg(amount_yen), sqlc.arg(confirmed_at))
RETURNING *;

-- name: InsertSupportAdvance :exec
INSERT INTO support_coach_advances (earning_id, contract_id, through_date, elapsed_days, total_days, amount_yen)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: SupportAdvanceTotals :one
-- 仮払いの合計（送金に含めた分と、まだの分）。
SELECT COALESCE(sum(a.amount_yen), 0)::int AS total_yen,
       COALESCE(sum(a.amount_yen) FILTER (WHERE e.payout_id IS NOT NULL), 0)::int AS paid_yen
FROM support_coach_advances a JOIN coach_earnings e ON e.id = a.earning_id
WHERE a.contract_id = $1;

-- name: DeleteUnpaidSupportAdvances :exec
-- コーチ都合で止めた契約の、まだ送金に含めていない仮払いを消す（送金の対象から外す）。
DELETE FROM coach_earnings e USING support_coach_advances a
WHERE a.earning_id = e.id AND a.contract_id = $1 AND e.payout_id IS NULL;

-- name: SetSupportEnd :one
UPDATE coaching_contracts
SET support_end_cause = sqlc.arg(cause), support_end_on = sqlc.arg(end_on),
    support_coach_hold = sqlc.arg(hold), updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: ReleaseSupportHold :one
UPDATE coaching_contracts SET support_end_cause = sqlc.arg(cause), support_coach_hold = false, updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: EndSupportWithoutRefund :one
-- 未経過分が無い（目標大会の日に終えた等）ときは、返金せずに終える。
UPDATE coaching_contracts SET status = 'ended', ends_at = LEAST(COALESCE(ends_at, now()), now()), updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListSupportFinalizations :many
-- 返金と精算が終わり、コーチ報酬をまだ確定していない契約（止めているコーチ都合のものは除く）。
SELECT ct.* FROM coaching_contracts ct
WHERE ct.fulfillment_version = 1 AND ct.video_reviews = 0 AND ct.service_type = 'competition_support'
  AND ct.status IN ('ended', 'refunded') AND ct.fulfillment_state = 'settled' AND NOT ct.support_coach_hold
  AND EXISTS (SELECT 1 FROM coaching_payments pay WHERE pay.contract_id = ct.id AND pay.status IN ('paid', 'partially_refunded', 'refunded')
              AND pay.refund_reason IS DISTINCT FROM 'not_started')
  AND NOT EXISTS (SELECT 1 FROM support_coach_settlements s WHERE s.contract_id = ct.id)
ORDER BY ct.updated_at
LIMIT 100;

-- name: GetSupportPaid :one
SELECT pay.amount_yen, pay.refunded_yen, pay.refunded_at FROM coaching_payments pay
WHERE pay.contract_id = $1 AND pay.status IN ('paid', 'partially_refunded', 'refunded')
ORDER BY pay.paid_at LIMIT 1;

-- name: InsertSupportFinalEarning :exec
INSERT INTO coach_earnings (id, coach_id, source, contract_id, gross_yen, share_percent, amount_yen, confirmed_at)
VALUES (sqlc.arg(id), sqlc.arg(coach_id), 'personal', sqlc.arg(contract_id), sqlc.arg(gross_yen), sqlc.arg(share_percent), sqlc.arg(amount_yen), sqlc.arg(confirmed_at));

-- name: InsertSupportSettlement :one
INSERT INTO support_coach_settlements (contract_id, earned_yen, advances_yen, final_yen, clawback_yen)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: GetSupportSettlement :one
SELECT * FROM support_coach_settlements WHERE contract_id = $1;

-- name: InsertSupportClawback :exec
INSERT INTO coach_refund_adjustments (id, contract_id, coach_id, kind, amount_yen)
VALUES ($1, $2, $3, 'support_clawback', $4);

-- name: InsertFaultDetermination :one
INSERT INTO support_fault_determinations (contract_id, decision, reason, decided_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetFaultDetermination :one
SELECT * FROM support_fault_determinations WHERE contract_id = $1;

-- name: ObjectFaultDetermination :one
-- 異議申立ては1回だけ。コーチ都合の判定に対して、まだ精算を確定していない間にだけ出せる。
UPDATE support_fault_determinations d SET objection = sqlc.arg(objection), objected_at = now()
WHERE d.contract_id = sqlc.arg(contract_id) AND d.decision = 'coach_fault' AND d.objected_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM support_coach_settlements s WHERE s.contract_id = d.contract_id)
RETURNING *;

-- name: ReviewFaultDetermination :one
UPDATE support_fault_determinations SET review_decision = sqlc.arg(decision), review_reason = sqlc.arg(reason),
    reviewed_by = sqlc.arg(reviewed_by), reviewed_at = now()
WHERE contract_id = sqlc.arg(contract_id) AND objected_at IS NOT NULL AND reviewed_at IS NULL
RETURNING *;

-- name: InsertContractStatement :exec
INSERT INTO support_contract_statements (contract_id, coach_id, body) VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING;

-- name: GetContractStatement :one
SELECT * FROM support_contract_statements WHERE contract_id = $1;
