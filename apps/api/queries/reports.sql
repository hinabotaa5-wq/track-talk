-- name: CreateReport :one
INSERT INTO reports (id, reporter_id, target_type, target_id, reason, detail)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, reporter_id, target_type, target_id, reason, detail, status, created_at;

-- name: CreateGuardianReport :one
-- 保護者からの通報。保護者はアカウントを持たないので、子のアカウントで by_guardian を立てる。
INSERT INTO reports (id, reporter_id, by_guardian, target_type, target_id, reason, detail)
VALUES ($1, $2, true, 'coaching_message', $3, $4, $5)
RETURNING id, reporter_id, target_type, target_id, reason, detail, status, created_at;

-- name: CountReports :one
SELECT count(*) FROM reports;

-- name: ListAdminReports :many
SELECT
    r.id,
    r.target_type,
    r.target_id,
    r.reason,
    r.detail,
    r.status,
    r.created_at,
    rp.nickname AS reporter_nickname,
    r.by_guardian,
    -- やりとり欄の通報は中身を出さず、契約の ID だけ返す（開くと記録が残る）。
    COALESCE(cm.contract_id, '00000000-0000-0000-0000-000000000000'::uuid)::uuid AS coaching_contract_id,
    COALESCE(p.id, c.post_id, vp.post_id, '00000000-0000-0000-0000-000000000000'::uuid)::uuid AS post_id,
    COALESCE(p.title, vpost.title, ''::text)::text AS target_title,
    COALESCE(p.body, c.body, n.note, rr.good_point, pr.comment, CASE WHEN r.target_type='voice_participant' THEN '音声は保存していません。対象: ' || COALESCE(vp.public_name,'記録期限終了') ELSE '' END)::text AS target_body,
    (COALESCE(p.hidden_at, c.hidden_at, n.hidden_at, rr.hidden_at, pr.hidden_at) IS NOT NULL)::boolean AS target_hidden,
    COALESCE(n.body_part, ''::text)::text AS injury_part
FROM reports r
JOIN profiles rp ON rp.id = r.reporter_id
LEFT JOIN meet_voice_participants vp ON r.target_type='voice_participant' AND vp.id=r.target_id
LEFT JOIN posts vpost ON vpost.id=vp.post_id
LEFT JOIN posts p ON r.target_type = 'post' AND p.id = r.target_id
LEFT JOIN comments c ON r.target_type = 'comment' AND c.id = r.target_id
LEFT JOIN injury_group_stage_log n ON r.target_type = 'injury_note' AND n.note_id = r.target_id
LEFT JOIN review_responses rr ON r.target_type = 'review_response' AND rr.request_id = r.target_id
LEFT JOIN coaching_messages cm ON r.target_type = 'coaching_message' AND cm.id = r.target_id
LEFT JOIN practice_ratings pr ON r.target_type = 'practice_rating' AND pr.id = r.target_id
ORDER BY r.created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListMyReportedTargets :many
SELECT target_type, target_id
FROM reports
WHERE reporter_id = sqlc.arg(reporter_id)
  AND (
    (target_type = 'post' AND target_id = sqlc.arg(post_id)::uuid)
    OR (target_type = 'comment' AND target_id IN (SELECT c.id FROM comments c WHERE c.post_id = sqlc.arg(post_id)::uuid))
  );

-- name: CloseReport :one
-- 運営が通報の対応を終える。やりとり欄の通報は、終えるまで保存期間を過ぎても会話を消さない。
UPDATE reports
SET status = 'reviewed'
WHERE id = $1 AND status = 'open'
RETURNING id, reporter_id, target_type, target_id, reason, detail, status, created_at;
