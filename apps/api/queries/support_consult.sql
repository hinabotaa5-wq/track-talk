-- name: GetSupportConsultSettings :one
SELECT * FROM support_consult_settings WHERE coach_id = $1;

-- name: SaveSupportConsultSettings :one
INSERT INTO support_consult_settings (coach_id, accepting, events, description, price_min_yen, price_max_yen)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (coach_id) DO UPDATE
SET accepting = EXCLUDED.accepting, events = EXCLUDED.events, description = EXCLUDED.description,
    price_min_yen = EXCLUDED.price_min_yen, price_max_yen = EXCLUDED.price_max_yen, updated_at = now()
RETURNING *;

-- name: ListPublicSupportConsultSettings :many
-- 相談の入口を出すコーチ。受付中か、受付停止中（一度でも受付を始めたコーチ）。承認済みで、いまの版の誓約をして、大会までプランを担当しているコーチだけ。
SELECT st.* FROM support_consult_settings st
JOIN coach_profiles c ON c.profile_id = st.coach_id
WHERE EXISTS (SELECT 1 FROM coaching_plans pl WHERE pl.coach_id = st.coach_id AND pl.consult_template)
  AND c.status = 'approved'
  AND c.pledge_version >= sqlc.arg(pledge_version)::smallint
  AND 'competition_support' = ANY(c.service_types)
  AND (sqlc.narg(coach_id)::uuid IS NULL OR st.coach_id = sqlc.narg(coach_id));

-- name: GetConsultTemplatePlan :one
SELECT * FROM coaching_plans WHERE coach_id = $1 AND consult_template;

-- name: CreateConsultTemplatePlan :one
-- 提案から契約を作るときの入れ物のプラン。公開中のままにしておく（公開を切り替えると特商法の表記の承認が外れるため）。
INSERT INTO coaching_plans (id, coach_id, kind, title, description, duration_days, price_yen, capacity, published, consult_template)
VALUES ($1, $2, 'period', '大会までプラン（無料相談の提案）', '無料相談のあと、コーチの提案に選手が申し込んだ大会までプランの契約です。期間・内容・価格は提案のとおりです。', 30, 2000, 50, true, true)
ON CONFLICT (coach_id) WHERE consult_template DO UPDATE SET coach_id = EXCLUDED.coach_id
RETURNING *;

-- name: CreateSupportConsult :one
INSERT INTO support_consults (id, athlete_id, coach_id, minor, event, answers, answer_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetSupportConsult :one
SELECT * FROM support_consults WHERE id = $1;

-- name: LockSupportConsult :one
SELECT * FROM support_consults WHERE id = $1 FOR UPDATE;

-- name: ListGuardianSupportConsults :many
-- 保護者のページに出す、子の無料相談のうち、保護者に提案が送られたもの。
SELECT sc.*, COALESCE(NULLIF(c.name, ''), p.nickname, '')::text AS coach_nickname, c.photo_public_id AS coach_photo_public_id
FROM support_consults sc
JOIN profiles p ON p.id = sc.coach_id
LEFT JOIN coach_profiles c ON c.profile_id = sc.coach_id
WHERE sc.athlete_id = $1 AND sc.minor AND sc.status IN ('sent_to_guardian', 'accepted')
ORDER BY sc.created_at DESC
LIMIT 50;

-- name: ListAthleteSupportConsults :many
SELECT sc.*, COALESCE(NULLIF(c.name, ''), p.nickname, '')::text AS coach_nickname, c.photo_public_id AS coach_photo_public_id
FROM support_consults sc
JOIN profiles p ON p.id = sc.coach_id
LEFT JOIN coach_profiles c ON c.profile_id = sc.coach_id
WHERE sc.athlete_id = $1
ORDER BY sc.created_at DESC
LIMIT 100;

-- name: ListCoachSupportConsults :many
SELECT sc.*, COALESCE(p.nickname, '')::text AS athlete_nickname
FROM support_consults sc
JOIN profiles p ON p.id = sc.athlete_id
WHERE sc.coach_id = $1
ORDER BY (sc.status IN ('requested', 'clarifying', 'proposed')) DESC, sc.created_at DESC
LIMIT 100;

-- name: HasOpenSupportConsult :one
SELECT EXISTS(SELECT 1 FROM support_consults WHERE athlete_id = $1 AND coach_id = $2 AND status IN ('requested', 'clarifying', 'proposed'))::boolean;

-- name: HasOpenCoachingContract :one
-- そのコーチとの契約が進行中か。進行中の契約があるあいだは、新しい無料相談は起こさない。
SELECT EXISTS(SELECT 1 FROM coaching_contracts WHERE requester_id = $1 AND coach_id = $2 AND status IN ('applied', 'accepted', 'active', 'suspended'))::boolean;

-- name: SetSupportConsultStatus :one
UPDATE support_consults
SET status = sqlc.arg(status)::text, answer_by = sqlc.narg(answer_by), updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: SetSupportConsultContract :one
UPDATE support_consults SET status = 'accepted', contract_id = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: CreateSupportConsultQuestion :one
INSERT INTO support_consult_questions (id, consult_id, question) VALUES ($1, $2, $3) RETURNING *;

-- name: ListSupportConsultQuestions :many
SELECT * FROM support_consult_questions WHERE consult_id = $1 ORDER BY asked_at, id;

-- name: AnswerSupportConsultQuestion :execrows
UPDATE support_consult_questions SET answer = $2, answered_at = now() WHERE consult_id = $1 AND answer IS NULL;

-- name: CountSupportConsultQuestions :one
SELECT count(*)::int FROM support_consult_questions WHERE consult_id = $1;

-- name: CreateSupportConsultProposal :one
INSERT INTO support_consult_proposals (consult_id, competition_name, support_start_date, competition_date, call_count, call_minutes, price_yen, content_note, answer_to_question, valid_until)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetSupportConsultProposal :one
SELECT * FROM support_consult_proposals WHERE consult_id = $1;

-- name: SnapshotSupportContractFromProposal :one
-- 契約の中身は、提案の写し。
UPDATE coaching_contracts ct
SET competition_name = pr.competition_name, competition_date = pr.competition_date, support_start_date = pr.support_start_date,
    call_count = pr.call_count, call_minutes = pr.call_minutes, fulfillment_version = 1
FROM support_consult_proposals pr
WHERE ct.id = sqlc.arg(contract_id) AND pr.consult_id = sqlc.arg(consult_id)
RETURNING ct.*;

-- name: ExpireSupportConsults :many
-- コーチが48時間返事をしなかった相談と、有効期限が過ぎた提案を閉じる。
UPDATE support_consults sc
SET status = 'expired', updated_at = now()
WHERE (sc.status = 'requested' AND sc.answer_by <= now())
   OR (sc.status IN ('proposed', 'sent_to_guardian') AND EXISTS (SELECT 1 FROM support_consult_proposals pr WHERE pr.consult_id = sc.id AND pr.valid_until <= now()))
RETURNING sc.*;

-- name: ListConsultTemplatePlans :many
SELECT id, coach_id FROM coaching_plans WHERE consult_template AND coach_id = ANY(sqlc.arg(coach_ids)::uuid[]);
