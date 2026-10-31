-- name: LockConsultationRoot :exec
SELECT pg_advisory_xact_lock(hashtext('consultation-root:' || sqlc.arg(root_id)::uuid::text));

-- name: LockConsultationContract :one
SELECT * FROM coaching_contracts WHERE id=$1 FOR UPDATE;

-- name: CountContractVideoSubmissions :one
SELECT count(*)::integer FROM coaching_messages WHERE contract_id=$1 AND sender_role='requester' AND video_public_id IS NOT NULL;

-- name: CreateConsultationDraft :one
INSERT INTO coaching_consultations (id,requester_id,coach_id,request_key,review_request_id,contract_id,minor,funding,price_yen,share_percent,question,video_public_id)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING *;

-- name: UpdateConsultationDraft :one
UPDATE coaching_consultations SET question=$2,video_public_id=$3,updated_at=now()
WHERE id=$1 AND status='draft' AND expires_at>now() AND purged_at IS NULL RETURNING *;

-- name: SetConsultationDraftFunding :one
-- 送信前の下書きの支払い方法(チケットか都度払いか)を、送信時の残高に合わせて変える。
UPDATE coaching_consultations SET funding=$2,updated_at=now()
WHERE id=$1 AND status='draft' AND expires_at>now() AND purged_at IS NULL RETURNING *;

-- name: GetConsultation :one
SELECT * FROM coaching_consultations WHERE id=$1;

-- name: LockConsultation :one
SELECT * FROM coaching_consultations WHERE id=$1 FOR UPDATE;

-- name: GetConsultationByKey :one
SELECT * FROM coaching_consultations WHERE requester_id=$1 AND request_key=$2;

-- name: ListRoomConsultations :many
SELECT * FROM coaching_consultations
WHERE review_request_id=sqlc.narg(review_request_id)::uuid OR contract_id=sqlc.narg(contract_id)::uuid
ORDER BY created_at,id;

-- name: ListCoachConsultations :many
SELECT c.id,c.review_request_id,c.contract_id,c.status,c.price_yen,c.respond_by,c.answer_by,c.clarify_requested_at,c.clarified_at,p.nickname
FROM coaching_consultations c LEFT JOIN profiles p ON p.id=c.requester_id
WHERE c.coach_id=$1 AND c.purged_at IS NULL
AND (c.status IN ('offered','in_review') OR (c.status='answered' AND c.clarify_requested_at IS NOT NULL AND c.clarified_at IS NULL))
ORDER BY c.created_at LIMIT 100;

-- name: SubmitTicketConsultation :one
UPDATE coaching_consultations SET status='offered',respond_by=now()+interval '48 hours',updated_at=now()
WHERE id=$1 AND status='draft' AND expires_at>now() RETURNING *;

-- name: StartCashConsultationPayment :one
UPDATE coaching_consultations SET status='awaiting_payment',expires_at=now()+interval '72 hours',updated_at=now()
WHERE id=$1 AND status='draft' AND expires_at>now() RETURNING *;

-- name: SetConsultationSessionExpiry :one
UPDATE coaching_consultations SET session_expires_at=$2,updated_at=now()
WHERE id=$1 AND status='awaiting_payment' AND checkout_session_id IS NULL RETURNING *;

-- name: SaveConsultationCheckout :exec
UPDATE coaching_consultations SET checkout_session_id=$2,checkout_url=$3,updated_at=now()
WHERE id=$1 AND (checkout_session_id IS NULL OR checkout_session_id=$2);

-- name: MarkConsultationPaid :one
UPDATE coaching_consultations SET payment_state='paid',payment_intent_id=$2,paid_at=now(),checkout_session_id=COALESCE(checkout_session_id,$3),
status=CASE WHEN status='awaiting_payment' AND expires_at>now() THEN 'offered' ELSE 'expired' END,
respond_by=now()+interval '48 hours',closed_at=CASE WHEN status='awaiting_payment' AND expires_at>now() THEN NULL ELSE now() END,updated_at=now()
WHERE id=$1 AND payment_state='unpaid' RETURNING *;

-- name: AcceptConsultation :one
UPDATE coaching_consultations SET status='in_review',answer_by=now()+interval '72 hours',updated_at=now()
WHERE id=$1 AND coach_id=$2 AND status='offered' AND respond_by>now() RETURNING *;

-- name: CloseConsultation :one
UPDATE coaching_consultations SET status=$2,closed_at=now(),updated_at=now()
WHERE id=$1 AND status IN ('draft','awaiting_payment','offered','in_review') RETURNING *;

-- name: AnswerConsultation :one
UPDATE coaching_consultations SET status='answered',answer=$2,answer_video_public_id=$3,answered_at=now(),closed_at=now(),updated_at=now()
WHERE id=$1 AND status='in_review' AND answer_by>now() AND hidden_at IS NULL RETURNING *;

-- name: RequestConsultationClarification :one
UPDATE coaching_consultations SET clarify_body=$2,clarify_requested_at=now(),updated_at=now()
WHERE id=$1 AND status='answered' AND answered_at>now()-interval '7 days' AND clarify_body IS NULL AND hidden_at IS NULL AND purged_at IS NULL RETURNING *;

-- name: AnswerConsultationClarification :one
UPDATE coaching_consultations SET clarification=$2,clarified_at=now(),updated_at=now()
WHERE id=$1 AND status='answered' AND clarify_body IS NOT NULL AND clarification IS NULL AND hidden_at IS NULL AND purged_at IS NULL RETURNING *;

-- name: CreateConsultationTicketReservation :one
INSERT INTO coaching_ticket_reservations (id,owner_id,consultation_id,quantity,nominal_yen,share_percent)
VALUES ($1,$2,$3,1,500,$4) RETURNING *;

-- name: GetConsultationTicketReservation :one
SELECT * FROM coaching_ticket_reservations WHERE consultation_id=$1;

-- name: RecordConsultationEarnings :exec
INSERT INTO coach_earnings (id,coach_id,source,consultation_id,gross_yen,share_percent,amount_yen)
SELECT gen_random_uuid(),c.coach_id,'consultation',c.id,c.price_yen,c.share_percent,(c.price_yen::bigint*c.share_percent/100)::integer
FROM coaching_consultations c WHERE c.id=$1 AND c.status='answered' AND c.coach_id IS NOT NULL AND (c.funding='ticket' OR c.payment_state='paid')
ON CONFLICT (consultation_id) DO NOTHING;

-- name: ListConsultationsToExpire :many
SELECT c.id FROM coaching_consultations c LEFT JOIN coaching_contracts ct ON ct.id=c.contract_id
LEFT JOIN coach_profiles cp ON cp.profile_id=c.coach_id
WHERE c.status IN ('draft','awaiting_payment','offered','in_review')
AND ((c.status IN ('draft','awaiting_payment') AND c.expires_at<=now())
 OR (c.status='awaiting_payment' AND c.session_expires_at<=now())
 OR (c.status='offered' AND c.respond_by<=now()) OR (c.status='in_review' AND c.answer_by<=now())
 OR c.requester_id IS NULL OR c.coach_id IS NULL OR cp.status IS DISTINCT FROM 'approved'
 OR (c.contract_id IS NOT NULL AND (ct.id IS NULL OR ct.status IN ('suspended','refunded'))))
ORDER BY c.updated_at LIMIT 100;

-- name: HasOpenConsultationAccount :one
SELECT (EXISTS(SELECT 1 FROM coaching_consultations c WHERE ($1=c.requester_id OR $1=c.coach_id)
 AND (c.status IN ('awaiting_payment','offered','in_review') OR c.payment_state='refund_pending' OR (c.clarify_requested_at IS NOT NULL AND c.clarified_at IS NULL AND c.hidden_at IS NULL AND c.purged_at IS NULL)))
 OR EXISTS(SELECT 1 FROM reports r JOIN coaching_consultations c ON r.target_id=c.id
 WHERE r.target_type='coaching_consultation' AND r.status='open' AND ($1=c.requester_id OR $1=c.coach_id)))::boolean;

-- name: CreateConsultationRefund :one
INSERT INTO coaching_consultation_refunds(id,consultation_id) VALUES ($1,$2) RETURNING *;

-- name: LockConsultationRefund :one
SELECT * FROM coaching_consultation_refunds WHERE id=$1 FOR UPDATE;

-- name: ListConsultationRefunds :many
SELECT * FROM coaching_consultation_refunds WHERE consultation_id=$1 ORDER BY created_at;

-- name: ListPendingConsultationRefunds :many
SELECT * FROM coaching_consultation_refunds WHERE status='pending' ORDER BY updated_at LIMIT 50;

-- name: SaveConsultationRefund :exec
UPDATE coaching_consultation_refunds SET status=$2,stripe_refund_id=$3,updated_at=now() WHERE id=$1;

-- name: TouchConsultationRefund :exec
UPDATE coaching_consultation_refunds SET updated_at=now() WHERE id=$1;

-- name: SetConsultationPaymentState :exec
UPDATE coaching_consultations SET payment_state=$2,updated_at=now() WHERE id=$1;

-- name: CreateGuardianConsultationReport :one
INSERT INTO reports(id,reporter_id,by_guardian,target_type,target_id,reason,detail)
VALUES ($1,$2,true,'coaching_consultation',$3,$4,$5) RETURNING *;

-- name: ConsultationHasReport :one
SELECT EXISTS(SELECT 1 FROM reports WHERE id=$1 AND target_type='coaching_consultation' AND target_id=$2)::boolean;

-- name: LogConsultationAccess :exec
INSERT INTO coaching_consultation_access_logs(id,admin_id,consultation_id,report_id) VALUES ($1,$2,$3,$4);

-- name: AdminConsultations :many
SELECT c.id,c.review_request_id,c.contract_id,c.status,c.funding,c.price_yen,c.payment_state,c.created_at,
 p.nickname AS requester_nickname,cp.nickname AS coach_nickname
FROM coaching_consultations c LEFT JOIN profiles p ON p.id=c.requester_id LEFT JOIN profiles cp ON cp.id=c.coach_id
WHERE c.status<>'draft' ORDER BY c.created_at DESC LIMIT 200;

-- name: ListConsultationsToPurge :many
SELECT c.* FROM coaching_consultations c WHERE c.purged_at IS NULL AND
((c.status IN ('draft','cancelled') AND c.payment_state='unpaid' AND c.expires_at<now()
  AND NOT EXISTS(SELECT 1 FROM coaching_ticket_reservations t WHERE t.consultation_id=c.id))
 OR (c.status IN ('answered','declined','expired','cancelled')
  AND GREATEST(c.closed_at,c.clarified_at,c.clarify_requested_at)<now()-interval '1 year'
  AND NOT EXISTS(SELECT 1 FROM coaching_contracts ct WHERE ct.id=c.contract_id
   AND (ct.status NOT IN ('ended','refunded') OR ct.ends_at>now()-interval '1 year'))
  AND NOT EXISTS(SELECT 1 FROM coaching_consultations sibling
   WHERE sibling.purged_at IS NULL AND sibling.id<>c.id
    AND (sibling.review_request_id=c.review_request_id OR sibling.contract_id=c.contract_id)
    AND (sibling.status IN ('awaiting_payment','offered','in_review')
      OR (sibling.status NOT IN ('draft','cancelled') AND GREATEST(sibling.closed_at,sibling.clarified_at,sibling.clarify_requested_at)>now()-interval '1 year')))))
AND NOT EXISTS(SELECT 1 FROM reports r JOIN coaching_consultations reported ON reported.id=r.target_id
 WHERE r.target_type='coaching_consultation' AND r.status='open'
 AND (reported.review_request_id=c.review_request_id OR reported.contract_id=c.contract_id))
AND NOT EXISTS(SELECT 1 FROM reports r JOIN coaching_messages m ON m.id=r.target_id
 WHERE r.target_type='coaching_message' AND m.contract_id=c.contract_id AND r.status='open')
ORDER BY c.updated_at LIMIT 20;

-- name: PurgeConsultation :exec
UPDATE coaching_consultations SET question='',answer='',video_public_id=NULL,answer_video_public_id=NULL,
 clarify_body=NULL,clarification=NULL,purged_at=now(),status=CASE WHEN status='draft' THEN 'cancelled' ELSE status END,
 closed_at=COALESCE(closed_at,now()) WHERE id=$1;

-- name: HideConsultation :exec
UPDATE coaching_consultations SET hidden_at=now(),updated_at=now() WHERE id=$1;

-- name: ListGuardianConsultationReviews :many
SELECT r.id,r.event,r.created_at FROM review_requests r
WHERE r.requester_id=$1 AND EXISTS(SELECT 1 FROM coaching_consultations c WHERE c.review_request_id=r.id AND c.minor AND c.status NOT IN ('draft','awaiting_payment'))
ORDER BY r.created_at DESC LIMIT 100;
