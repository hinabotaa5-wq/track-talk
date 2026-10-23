-- name: SaveInPersonPlan :one
UPDATE coaching_plans SET venue_area=$2,venue_name=$3,session_count=$4,session_minutes=$5,travel_fee_yen=$6,max_participants=$7 WHERE id=$1 RETURNING *;

-- name: SnapshotInPersonContract :one
UPDATE coaching_contracts ct SET venue_area=pl.venue_area,venue_name=pl.venue_name,session_count=pl.session_count,session_minutes=pl.session_minutes,travel_fee_yen=pl.travel_fee_yen,max_participants=pl.max_participants,price_yen=pl.price_yen+pl.travel_fee_yen,fulfillment_version=2
FROM coaching_plans pl WHERE ct.id=$1 AND pl.id=ct.plan_id AND ct.service_type='in_person' RETURNING ct.*;

-- name: InPersonUsage :one
SELECT (SELECT count(*)::int FROM coaching_in_person_sessions s WHERE s.contract_id=$1 AND s.status IN ('proposed','confirmed','completion_pending')) AS reserved,
 (SELECT CASE WHEN ct.fulfillment_state='settled' THEN ct.delivered_session_count ELSE (SELECT count(*)::int FROM coaching_in_person_sessions s WHERE s.contract_id=ct.id AND s.status='completed') END FROM coaching_contracts ct WHERE ct.id=$1)::int AS completed;

-- name: InPersonOverlap :one
SELECT EXISTS(SELECT 1 FROM coaching_in_person_sessions WHERE status IN ('proposed','confirmed','completion_pending')
 AND (coach_id=sqlc.arg(coach_id) OR requester_id=sqlc.arg(requester_id))
 AND starts_at<sqlc.arg(ends_at)::timestamptz AND ends_at>sqlc.arg(starts_at)::timestamptz)::boolean;

-- name: GetInPersonSessionByKey :one
SELECT * FROM coaching_in_person_sessions WHERE contract_id=$1 AND request_key=$2;

-- name: CreateInPersonSession :one
INSERT INTO coaching_in_person_sessions(id,contract_id,coach_id,requester_id,request_key,starts_at,ends_at,venue_name,venue_area,meeting_note,participants)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING *;

-- name: GetInPersonSession :one
SELECT * FROM coaching_in_person_sessions WHERE id=$1 AND contract_id=$2;

-- name: ListInPersonSessions :many
SELECT * FROM coaching_in_person_sessions WHERE contract_id=$1 ORDER BY starts_at,id;

-- name: ChangeInPersonSession :one
UPDATE coaching_in_person_sessions SET status=sqlc.arg(status)::text,
 reported_at=CASE WHEN sqlc.arg(status)::text='completion_pending' THEN now() ELSE reported_at END,
 completed_at=CASE WHEN sqlc.arg(status)::text='completed' THEN now() ELSE completed_at END,
 cancelled_at=CASE WHEN sqlc.arg(status)::text='cancelled' THEN now() ELSE cancelled_at END,updated_at=now()
WHERE id=sqlc.arg(id) RETURNING *;

-- name: CancelInPersonSessions :exec
UPDATE coaching_in_person_sessions SET status='cancelled',cancelled_at=now(),updated_at=now()
WHERE contract_id=$1 AND status IN ('proposed','confirmed','completion_pending');

-- name: MarkInPersonSettled :exec
UPDATE coaching_contracts SET fulfillment_state='settled',delivered_session_count=(SELECT count(*)::int FROM coaching_in_person_sessions s WHERE s.contract_id=coaching_contracts.id AND s.status='completed'),updated_at=now() WHERE coaching_contracts.id=$1;

-- name: ListInPersonSettlements :many
SELECT ct.id FROM coaching_contracts ct WHERE ct.fulfillment_version=2
 AND (ct.fulfillment_state='refund_pending' OR (ct.fulfillment_state='open' AND ct.status='ended' AND ct.ends_at<now()-interval '48 hours'))
ORDER BY ct.updated_at LIMIT 50;
