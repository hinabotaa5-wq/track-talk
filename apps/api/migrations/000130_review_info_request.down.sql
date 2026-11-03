DELETE FROM notifications WHERE kind IN ('review_info_request', 'review_resubmitted');
ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications
    ADD CONSTRAINT notifications_kind_check CHECK (kind IN (
        'recruitment_post',
        'comment',
        'comment_reply',
        'practice_application',
        'practice_accepted',
        'practice_rate',
        'injury_update',
        'review_offer',
        'review_expired',
        'review_returned',
        'review_clarify',
        'review_clarified',
        'review_helpful_ask',
        'message'
    ));

-- 差し戻し中の依頼は、戻す先の状態がないので期限切れにする（支払いは tick が取り消す）。
UPDATE review_requests SET status = 'expired', updated_at = now() WHERE status = 'needs_info';
DROP INDEX review_requests_needs_info_idx;
ALTER TABLE review_requests
    DROP CONSTRAINT review_requests_info_check,
    DROP COLUMN info_request,
    DROP COLUMN info_reason,
    DROP COLUMN info_requested_at,
    DROP COLUMN info_from_status,
    DROP COLUMN resubmit_by,
    DROP COLUMN original_question,
    DROP COLUMN video_replaced,
    DROP COLUMN resubmitted_at,
    DROP CONSTRAINT review_requests_status_check,
    ADD CONSTRAINT review_requests_status_check CHECK (status IN (
        'awaiting_payment', 'queued', 'offered', 'in_review', 'returned', 'clarifying', 'expired'
    ));
