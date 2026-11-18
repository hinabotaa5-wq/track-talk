-- name: InsertReviewFeedback :one
-- 返却された添削への匿名の感想（本体 #911）。1添削につき1件まで（request_id の UNIQUE）。
-- 送信から7日たつまでコーチには見せない。
INSERT INTO review_feedbacks (id, request_id, coach_id, body, visible_at)
VALUES (sqlc.arg(id), sqlc.arg(request_id), sqlc.arg(coach_id), sqlc.arg(body), now() + interval '7 days')
ON CONFLICT (request_id) DO NOTHING
RETURNING visible_at;

-- name: ReviewFeedbackSubmitted :one
-- 選手本人に、その依頼へ感想を送ったかどうかだけを返す。
SELECT EXISTS (
    SELECT 1 FROM review_feedbacks WHERE request_id = sqlc.arg(request_id)
)::boolean;

-- name: ListVisibleCoachFeedbacks :many
-- コーチに見せる感想。表示開始（送信から7日）を過ぎた自分宛てのものだけ。
-- どの依頼・どの選手かが分からないよう、id と本文しか返さない。
SELECT id, body
FROM review_feedbacks
WHERE coach_id = sqlc.arg(coach_id)
  AND visible_at <= now()
ORDER BY visible_at DESC, id DESC
LIMIT 200;

-- name: MarkReviewFeedbacksNotified :many
-- 表示開始を過ぎて、まだ知らせていない感想に印をつけ、知らせる先のコーチを返す。tick から呼ぶ。
-- 印をつけた行だけを返すので、tick が重なっても同じ感想を二度知らせない。
UPDATE review_feedbacks
SET notified_at = now()
WHERE visible_at <= now()
  AND notified_at IS NULL
RETURNING coach_id;
