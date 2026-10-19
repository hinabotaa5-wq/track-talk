-- name: ReleaseCommentThreadsForUser :exec
UPDATE comments
SET parent_id = NULL
WHERE parent_id IN (
    SELECT c.id
    FROM comments c
    WHERE c.author_id = sqlc.arg('user_id')
       OR c.post_id IN (SELECT id FROM posts WHERE author_id = sqlc.arg('user_id'))
);

-- name: DecrementCommentCountsForUser :exec
UPDATE posts AS p
SET comment_count = GREATEST(p.comment_count - removed.n, 0),
    updated_at = now()
FROM (
    SELECT post_id, count(*)::int AS n
    FROM comments
    WHERE author_id = sqlc.arg('user_id')
      AND deleted_at IS NULL
    GROUP BY post_id
) AS removed
WHERE p.id = removed.post_id
  AND p.author_id <> sqlc.arg('user_id');

-- name: DecrementLikeCountsForUser :exec
UPDATE posts AS p
SET like_count = GREATEST(p.like_count - 1, 0),
    updated_at = now()
WHERE p.id IN (
    SELECT post_id FROM post_likes WHERE user_id = sqlc.arg('user_id')
)
  AND p.author_id <> sqlc.arg('user_id');

-- name: DeletePracticeRatingsForUser :exec
DELETE FROM practice_ratings
WHERE rater_id = sqlc.arg('user_id')
   OR ratee_id = sqlc.arg('user_id');

-- name: DeletePracticeApplicationsForUser :exec
DELETE FROM practice_applications
WHERE user_id = sqlc.arg('user_id');

-- name: DeleteReportsForUser :exec
DELETE FROM reports
WHERE reporter_id = sqlc.arg('user_id');

-- name: DeleteCommentsForUser :exec
DELETE FROM comments
WHERE author_id = sqlc.arg('user_id');

-- name: DeletePostsForUser :exec
DELETE FROM posts
WHERE author_id = sqlc.arg('user_id');

-- name: DeleteProfile :exec
DELETE FROM profiles
WHERE id = sqlc.arg('user_id');

-- name: WithdrawBlockers :one
-- 退会すると契約とやりとりが消えるので、進行中のパーソナル指導と、
-- 自分の契約のやりとりへの未解決の通報があるあいだは退会させない。
SELECT
    EXISTS (
        SELECT 1 FROM coaching_contracts ct
        WHERE (ct.coach_id = sqlc.arg('user_id') OR ct.requester_id = sqlc.arg('user_id'))
          AND ct.status IN ('applied', 'accepted', 'active', 'suspended')
    )::boolean AS open_contract,
    EXISTS (
        SELECT 1 FROM reports rp
        JOIN coaching_messages m ON m.id = rp.target_id
        JOIN coaching_contracts ct ON ct.id = m.contract_id
        WHERE rp.target_type = 'coaching_message'
          AND rp.status = 'open'
          AND (ct.coach_id = sqlc.arg('user_id') OR ct.requester_id = sqlc.arg('user_id'))
    )::boolean AS open_report,
    -- 依頼は退会で消え、依頼が消えた支払いは tick が返金する。確定していない支払いがあるあいだは退会させない。
    -- 確定済み（返却された依頼の captured）だけになれば退会できる。
    EXISTS (
        SELECT 1 FROM review_payments pay
        LEFT JOIN review_requests r ON r.id = pay.request_id
        WHERE pay.payer_id = sqlc.arg('user_id')
          AND pay.status IN ('authorized', 'captured')
          AND (pay.status = 'authorized' OR r.id IS NULL OR r.returned_at IS NULL)
    )::boolean AS open_review_payment;
