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

-- name: RememberDeletedAccount :exec
INSERT INTO deleted_accounts (id) VALUES (sqlc.arg('user_id'))
ON CONFLICT (id) DO NOTHING;

-- name: RetireCoachProfileForWithdraw :exec
-- 退会の削除順の修正（#730）。契約の先行削除の前に、コーチの status を終了側へ
-- 1回だけ動かし、契約がまだある状態で identity_retirement_status トリガーに
-- identity_boundary.mark_retired を実行させ、終了済み契約の updated_at を
-- retirement.last_transaction_at の起算に含める。
-- status と on_duty=false は同じ UPDATE で書く（coach_profiles_on_duty_check
-- 回避。WithdrawCoach／SuspendCoach／EndSuspendedCoach と同じ形）。
-- photo_public_id には触れない（guard_retired_media を発火させない）。
-- 同じ更新で identity_coach_change が epoch を付け替えるが、同一トランザクション内の
-- 中間状態で、直後の DeleteCoachProfileForUser で coach_epochs 行ごと消えるため無害。
-- approved／pending へは戻さない（retirement 行を消す分岐を踏まない）。
UPDATE coach_profiles
SET status = CASE status
    WHEN 'approved' THEN 'withdrawn'
    WHEN 'pending' THEN 'withdrawn'
    WHEN 'suspended' THEN 'withdrawn'
    WHEN 'withdrawn' THEN 'ended'
    WHEN 'rejected' THEN 'ended'
    WHEN 'ended' THEN 'withdrawn'
    ELSE status
END,
    on_duty = false,
    updated_at = now()
WHERE profile_id = sqlc.arg('user_id');

-- name: DeleteCoachingContractsForCoach :exec
-- 退会の削除順の修正（#730）。coach_profiles を消す前に、そのコーチの契約を先に消す。
-- 契約は DeleteProfile 時の coach_id の ON DELETE CASCADE でも消えるもの（最終状態は同じ）。
-- 先に消す理由：coach_profiles 削除からの coaching_plans の CASCADE 削除の途中で、
-- coaching_contracts.plan_id の ON DELETE RESTRICT に当たり 23503 になるため。
-- 契約にぶら下がる明細・同意・通報・精算記録は、契約側の CASCADE／SET NULL に従う
-- （精算記録4表と coaching_payments は #729 で SET NULL のまま残る）。
DELETE FROM coaching_contracts
WHERE coach_id = sqlc.arg('user_id');

-- name: DeleteCoachProfileForUser :exec
-- 退会でコーチ行を確実に消す。coach_profiles.profile_id は profiles(id) への
-- ON DELETE CASCADE でもあるが、CASCADE 任せにせず DeleteProfile の直前で明示的に消す。
DELETE FROM coach_profiles
WHERE profile_id = sqlc.arg('user_id');

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
          AND (ct.status IN ('applied', 'accepted', 'active', 'suspended') OR (ct.fulfillment_version=1 AND ct.fulfillment_state<>'settled' AND ct.status IN ('ended','refunded')))
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
          AND pay.status IN ('authorized', 'captured', 'partially_refunded', 'refund_pending')
          AND (pay.status IN ('authorized','refund_pending') OR r.id IS NULL OR r.returned_at IS NULL)
    )::boolean AS open_review_payment,
    -- コーチとして割り当てられた対応中の依頼は、退会で担当不在になるので、返却・終了まで退会させない。
    -- 支払い種別（カード／PayPay／チケット）を問わず、review_requests の状態だけで見る（CoachWithdrawBlockers の open_review と同じ条件）。
    EXISTS (
        SELECT 1 FROM review_requests r
        WHERE r.coach_id = sqlc.arg('user_id')
          AND r.status IN ('in_review', 'needs_info')
    )::boolean AS open_review;

-- name: ListAccountMedia :many
-- 退会で消える行が使っている Cloudinary の写真・動画。resource は image / video / coaching（非公開の添削動画）。
SELECT 'image'::text AS resource, p.cloudinary_public_id::text AS public_id
FROM posts p WHERE p.author_id = sqlc.arg('user_id') AND p.kind IN ('text', 'instagram') AND p.cloudinary_public_id IS NOT NULL
UNION ALL
SELECT 'video'::text, p.cloudinary_public_id::text
FROM posts p WHERE p.author_id = sqlc.arg('user_id') AND p.kind = 'video' AND p.cloudinary_public_id IS NOT NULL
UNION ALL
SELECT 'image'::text, rp.cloudinary_public_id
FROM recruitment_photos rp JOIN posts p ON p.id = rp.post_id WHERE p.author_id = sqlc.arg('user_id')
UNION ALL
SELECT 'image'::text, cp.photo_public_id::text
FROM coach_profiles cp WHERE cp.profile_id = sqlc.arg('user_id') AND cp.photo_public_id IS NOT NULL
UNION ALL
SELECT 'coaching'::text, r.video_public_id
FROM review_requests r WHERE r.requester_id = sqlc.arg('user_id') AND r.video_public_id IS NOT NULL
UNION ALL
-- プレミアムの動画（本体 #1053）。依頼と一緒に消えるので、依頼の動画と同じく退会で消す。
SELECT 'coaching'::text, v.video_public_id
FROM review_request_videos v JOIN review_requests r ON r.id = v.request_id WHERE r.requester_id = sqlc.arg('user_id')
UNION
-- コーチ詳細の見本動画（本体 #1086）。申請ごとコーチと一緒に消えるので消す。ただし同じ動画を回答の見本動画にも使っていて、
-- その回答が残る（選手の依頼は消えない）ときは消さない。
SELECT 'coaching'::text, v.video_public_id
FROM coach_offer_application_videos v JOIN coach_offer_applications a ON a.id = v.application_id
WHERE a.coach_id = sqlc.arg('user_id')
  AND NOT EXISTS (SELECT 1 FROM review_responses rs WHERE rs.demo_video_public_id = v.video_public_id);
