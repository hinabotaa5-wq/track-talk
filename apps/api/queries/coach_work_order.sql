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

-- name: InsertWorkOrderNotification :exec
-- 取引条件を届けるお知らせ。法律上の明示にあたるので、本人の通知の設定に関わらず、まとめずに必ず1件入れる。
INSERT INTO notifications (id, recipient_id, kind, title, body, url)
VALUES ($1, $2, 'message', $3, $4, $5);

-- name: CreateWorkOrderDelivery :one
-- 届けた記録(宛先・日時・送った文面)。同じ通知を2回届けない。
INSERT INTO coach_work_order_deliveries (id, work_order_id, recipient_id, channel, terms_version, title, body, url)
VALUES ($1, $2, $3, 'in_app', $4, $5, $6, $7)
RETURNING *;

-- name: GetWorkOrderDelivery :one
SELECT * FROM coach_work_order_deliveries WHERE work_order_id = $1;
