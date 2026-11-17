-- 海外コーチとのやり取りの自動翻訳（本体 #916）。対象はコーチの profiles.locale が 'en' のペアだけ。

-- name: GetProfileLocale :one
SELECT locale FROM profiles WHERE id = $1;

-- name: GetReviewTranslation :one
SELECT * FROM review_translations WHERE request_id = $1;

-- name: UpsertReviewQuestionTranslation :exec
INSERT INTO review_translations (request_id, question_en, question_source_hash)
VALUES (sqlc.arg(request_id), sqlc.arg(question_en), sqlc.arg(source_hash)::text)
ON CONFLICT (request_id) DO UPDATE
SET question_en = EXCLUDED.question_en, question_source_hash = EXCLUDED.question_source_hash, updated_at = now();

-- name: UpsertReviewResponseTranslation :exec
INSERT INTO review_translations (request_id, good_point_ja, fix_points_ja, drill_ja, response_source_hash)
VALUES (sqlc.arg(request_id), sqlc.arg(good_point_ja), sqlc.arg(fix_points_ja)::text[], sqlc.arg(drill_ja), sqlc.arg(source_hash)::text)
ON CONFLICT (request_id) DO UPDATE
SET good_point_ja = EXCLUDED.good_point_ja, fix_points_ja = EXCLUDED.fix_points_ja, drill_ja = EXCLUDED.drill_ja,
    response_source_hash = EXCLUDED.response_source_hash, updated_at = now();

-- name: UpsertReviewInfoRequestTranslation :exec
INSERT INTO review_translations (request_id, info_request_ja, info_request_source_hash)
VALUES (sqlc.arg(request_id), sqlc.arg(info_request_ja), sqlc.arg(source_hash)::text)
ON CONFLICT (request_id) DO UPDATE
SET info_request_ja = EXCLUDED.info_request_ja, info_request_source_hash = EXCLUDED.info_request_source_hash, updated_at = now();

-- name: ListConsultationTranslations :many
SELECT * FROM consultation_translations WHERE consultation_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: UpsertConsultationQuestionTranslation :exec
INSERT INTO consultation_translations (consultation_id, question_en, question_source_hash)
VALUES (sqlc.arg(consultation_id), sqlc.arg(text), sqlc.arg(source_hash)::text)
ON CONFLICT (consultation_id) DO UPDATE
SET question_en = EXCLUDED.question_en, question_source_hash = EXCLUDED.question_source_hash, updated_at = now();

-- name: UpsertConsultationAnswerTranslation :exec
INSERT INTO consultation_translations (consultation_id, answer_ja, answer_source_hash)
VALUES (sqlc.arg(consultation_id), sqlc.arg(text), sqlc.arg(source_hash)::text)
ON CONFLICT (consultation_id) DO UPDATE
SET answer_ja = EXCLUDED.answer_ja, answer_source_hash = EXCLUDED.answer_source_hash, updated_at = now();

-- name: UpsertConsultationClarifyTranslation :exec
INSERT INTO consultation_translations (consultation_id, clarify_body_en, clarify_source_hash)
VALUES (sqlc.arg(consultation_id), sqlc.arg(text), sqlc.arg(source_hash)::text)
ON CONFLICT (consultation_id) DO UPDATE
SET clarify_body_en = EXCLUDED.clarify_body_en, clarify_source_hash = EXCLUDED.clarify_source_hash, updated_at = now();

-- name: UpsertConsultationClarificationTranslation :exec
INSERT INTO consultation_translations (consultation_id, clarification_ja, clarification_source_hash)
VALUES (sqlc.arg(consultation_id), sqlc.arg(text), sqlc.arg(source_hash)::text)
ON CONFLICT (consultation_id) DO UPDATE
SET clarification_ja = EXCLUDED.clarification_ja, clarification_source_hash = EXCLUDED.clarification_source_hash, updated_at = now();

-- name: ListCoachingMessageTranslations :many
SELECT * FROM coaching_message_translations WHERE message_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: UpsertCoachingMessageTranslation :exec
INSERT INTO coaching_message_translations (message_id, body_en, body_ja, menu_ja, source_hash)
VALUES (sqlc.arg(message_id), sqlc.arg(body_en), sqlc.arg(body_ja), sqlc.narg(menu_ja)::jsonb, sqlc.arg(source_hash))
ON CONFLICT (message_id) DO UPDATE
SET body_en = EXCLUDED.body_en, body_ja = EXCLUDED.body_ja, menu_ja = EXCLUDED.menu_ja, source_hash = EXCLUDED.source_hash, updated_at = now();

-- name: ListEnCoachReviewRequests :many
-- backfill の対象：担当コーチの locale が 'en' の添削依頼。
SELECT r.* FROM review_requests r
JOIN profiles p ON p.id = r.coach_id
WHERE p.locale = 'en'
ORDER BY r.created_at;

-- name: ListEnCoachConsultations :many
-- backfill の対象：担当コーチの locale が 'en' の追加質問。下書き・隠した・消した相談は除く。
SELECT c.* FROM coaching_consultations c
JOIN profiles p ON p.id = c.coach_id
WHERE p.locale = 'en'
  AND c.status <> 'draft'
  AND c.hidden_at IS NULL
  AND c.purged_at IS NULL
ORDER BY c.created_at;

-- name: ListEnCoachMessages :many
-- backfill の対象：契約のコーチの locale が 'en' のやりとり欄のメッセージ。
SELECT m.* FROM coaching_messages m
JOIN coaching_contracts ct ON ct.id = m.contract_id
JOIN profiles p ON p.id = ct.coach_id
WHERE p.locale = 'en'
ORDER BY m.created_at;
