-- #898: 継続プランのコーチ不在日の合意延長。

-- name: GetSupportConsultByContract :one
-- 1ヶ月プランの契約になった無料相談（目標の大会日は回答に入っている）。
SELECT * FROM support_consults WHERE contract_id = $1;

-- name: InsertAbsenceExtension :one
INSERT INTO coaching_absence_extensions (
    contract_id, coach_id, requester_id, absent_from, return_on, absent_days, reason, minor, price_yen,
    original_ends_at, proposed_ends_at, refund_estimate_yen
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: GetAbsenceExtensionForUpdate :one
SELECT * FROM coaching_absence_extensions WHERE id = $1 AND contract_id = $2 FOR UPDATE;

-- name: ListAbsenceExtensions :many
SELECT * FROM coaching_absence_extensions WHERE contract_id = $1 ORDER BY declared_at, id;

-- name: SetAbsencePlayerConsent :one
UPDATE coaching_absence_extensions SET player_consented_by = $2, player_consented_at = now()
WHERE id = $1 AND status = 'declared' RETURNING *;

-- name: SetAbsenceGuardianConsent :one
UPDATE coaching_absence_extensions SET guardian_consented_at = now()
WHERE id = $1 AND status = 'declared' AND minor RETURNING *;

-- name: SetAbsenceCoachConsent :one
UPDATE coaching_absence_extensions SET coach_consented_by = $2, coach_consented_at = now()
WHERE id = $1 AND status = 'declared' RETURNING *;

-- name: CloseAbsenceExtension :one
UPDATE coaching_absence_extensions SET status = $2, closed_at = now()
WHERE id = $1 AND status = 'declared' RETURNING *;

-- name: AgreeAbsenceExtension :one
UPDATE coaching_absence_extensions SET
    status = 'agreed', agreed_at = now(), agreed_original_ends_at = $2, agreed_new_ends_at = $3, coach_terms = $4
WHERE id = $1 AND status = 'declared' RETURNING *;

-- name: ApplyContractAbsence :one
-- 合意が成立したときだけ呼ぶ。終了日を不在日数分だけ後ろ倒しする（価格・内容・分母の期間日数は変えない）。
UPDATE coaching_contracts SET
    ends_at = ends_at + make_interval(days => sqlc.arg(days)::int),
    absence_from = sqlc.arg(absent_from), absence_return_on = sqlc.arg(return_on), absence_days = sqlc.arg(days)::int,
    updated_at = now()
WHERE id = sqlc.arg(id) AND status = 'active' AND absence_days = 0
RETURNING *;

-- name: CreateSystemCoachingMessage :one
-- 契約チャットに残す不在の申告・合意の記録。コーチの返信とは数えない（system_kind が付く）。
INSERT INTO coaching_messages (id, contract_id, sender_id, sender_role, body, system_kind)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListLapsedAbsenceExtensions :many
-- 復帰予定日を過ぎても合意がそろわなかった申告。
SELECT * FROM coaching_absence_extensions WHERE status = 'declared' AND return_on <= $1::date ORDER BY declared_at LIMIT 100;
