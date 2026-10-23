-- name: SaveSupportPlan :one
UPDATE coaching_plans SET competition_name=$2,competition_date=$3,support_start_date=$4,call_count=$5,call_minutes=$6 WHERE id=$1 RETURNING *;

-- name: SnapshotSupportContract :one
UPDATE coaching_contracts ct SET competition_name=pl.competition_name,competition_date=pl.competition_date,
 support_start_date=pl.support_start_date,call_count=pl.call_count,call_minutes=pl.call_minutes,fulfillment_version=1
FROM coaching_plans pl WHERE ct.id=$1 AND pl.id=ct.plan_id AND ct.service_type='competition_support' RETURNING ct.*;

-- name: SupportUsage :one
SELECT (SELECT count(*)::int FROM coaching_video_reviews v WHERE v.contract_id=$1) AS video_submitted,
 (SELECT CASE WHEN ct.fulfillment_state='settled' THEN ct.delivered_video_count ELSE (SELECT count(*)::int FROM coaching_video_reviews v WHERE v.contract_id=ct.id AND v.answer_id IS NOT NULL) END FROM coaching_contracts ct WHERE ct.id=$1)::int AS video_answered,
 (SELECT count(*)::int FROM coaching_calls c WHERE c.contract_id=$1 AND c.status IN ('proposed','confirmed','completion_pending')) AS calls_reserved,
 (SELECT CASE WHEN ct.fulfillment_state='settled' THEN ct.delivered_call_count ELSE (SELECT count(*)::int FROM coaching_calls c WHERE c.contract_id=ct.id AND c.status='completed') END FROM coaching_contracts ct WHERE ct.id=$1)::int AS calls_completed;

-- name: CreateSupportVideo :exec
INSERT INTO coaching_video_reviews(question_id,contract_id) VALUES ($1,$2);

-- name: AnswerSupportVideo :execrows
UPDATE coaching_video_reviews SET answer_id=$3,answered_at=now() WHERE question_id=$1 AND contract_id=$2 AND answer_id IS NULL;

-- name: ListSupportVideos :many
SELECT question_id,answer_id,answered_at,created_at FROM coaching_video_reviews WHERE contract_id=$1 ORDER BY created_at;

-- name: SetMessageReviewSubmission :exec
UPDATE coaching_messages SET review_submission_id=$2 WHERE id=$1;

-- name: LockCallCalendar :exec
SELECT pg_advisory_xact_lock(hashtext('coaching-call-calendar'));

-- name: SupportCallOverlap :one
SELECT EXISTS(SELECT 1 FROM coaching_calls WHERE status IN ('proposed','confirmed','completion_pending')
 AND (coach_id=sqlc.arg(coach_id) OR requester_id=sqlc.arg(requester_id))
 AND starts_at<sqlc.arg(ends_at)::timestamptz AND ends_at>sqlc.arg(starts_at)::timestamptz)::boolean;

-- name: GetSupportCallByKey :one
SELECT * FROM coaching_calls WHERE contract_id=$1 AND request_key=$2;

-- name: CreateSupportCall :one
INSERT INTO coaching_calls(id,contract_id,coach_id,requester_id,request_key,starts_at,ends_at,join_url)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING *;

-- name: GetSupportCall :one
SELECT * FROM coaching_calls WHERE id=$1 AND contract_id=$2;

-- name: ListSupportCalls :many
SELECT * FROM coaching_calls WHERE contract_id=$1 ORDER BY starts_at,id;

-- name: ChangeSupportCall :one
UPDATE coaching_calls SET status=sqlc.arg(status)::text,
 reported_at=CASE WHEN sqlc.arg(status)::text='completion_pending' THEN now() ELSE reported_at END,
 completed_at=CASE WHEN sqlc.arg(status)::text='completed' THEN now() ELSE completed_at END,
 cancelled_at=CASE WHEN sqlc.arg(status)::text='cancelled' THEN now() ELSE cancelled_at END,updated_at=now()
WHERE id=sqlc.arg(id) RETURNING *;

-- name: CancelSupportCalls :exec
UPDATE coaching_calls SET status='cancelled',cancelled_at=now(),updated_at=now()
WHERE contract_id=$1 AND status IN ('proposed','confirmed','completion_pending');

-- name: MarkSupportSettled :exec
UPDATE coaching_contracts SET fulfillment_state='settled',delivered_video_count=(SELECT count(*)::int FROM coaching_video_reviews v WHERE v.contract_id=coaching_contracts.id AND v.answer_id IS NOT NULL),delivered_call_count=(SELECT count(*)::int FROM coaching_calls c WHERE c.contract_id=coaching_contracts.id AND c.status='completed'),updated_at=now() WHERE coaching_contracts.id=$1;

-- name: PrepareSupportRefund :one
UPDATE coaching_contracts SET fulfillment_state='refund_pending',support_refund_amount_yen=$2,support_refund_attempt_id=$3,
 support_refund_reason=$4,status='refunded',ends_at=LEAST(COALESCE(ends_at,now()),now()),updated_at=now()
WHERE id=$1 RETURNING *;

-- name: FailSupportRefund :exec
UPDATE coaching_contracts SET fulfillment_state='refund_failed',updated_at=now() WHERE id=$1;

-- name: ListSupportSettlements :many
SELECT ct.id FROM coaching_contracts ct WHERE ct.fulfillment_version=1
 AND (ct.fulfillment_state='refund_pending' OR (ct.fulfillment_state='open' AND
 ((ct.status='ended' AND ct.ends_at<now()-interval '48 hours')
 OR (ct.status='active' AND ct.payment_required AND ct.starts_at<now()-interval '72 hours'
  AND NOT EXISTS(SELECT 1 FROM coaching_messages m WHERE m.contract_id=ct.id AND m.sender_role='coach')
  AND NOT EXISTS(SELECT 1 FROM coaching_calls c WHERE c.contract_id=ct.id AND c.status<>'cancelled')))))
ORDER BY ct.updated_at LIMIT 50;

-- name: PurgeSupportLinks :exec
UPDATE coaching_calls SET join_url='' WHERE contract_id=$1;
