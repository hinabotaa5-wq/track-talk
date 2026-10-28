-- name: CreateCoachWorkOrder :one
-- 同じ案件の通知は1回だけ。引き受け直しなどで2回目が来たら、最初の通知を残す。
INSERT INTO coach_work_orders (id, kind, review_request_id, consultation_id, coach_id, terms_version, terms)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: GetReviewWorkOrder :one
SELECT * FROM coach_work_orders WHERE review_request_id = $1 AND coach_id = $2;

-- name: GetConsultationWorkOrder :one
SELECT * FROM coach_work_orders WHERE consultation_id = $1 AND coach_id = $2;

-- name: GetReviewTicketTerms :one
-- チケットで払った添削の、割引前の価格と報酬率(依頼のときに確定したもの)。
SELECT nominal_yen, share_percent FROM coaching_ticket_reservations WHERE request_id = $1;
