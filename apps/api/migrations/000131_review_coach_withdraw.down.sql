DELETE FROM notifications WHERE kind = 'review_withdrawn';
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
        'review_info_request',
        'review_resubmitted',
        'message'
    ));

-- 辞退した依頼は expired のまま残る（支払いは取り消し・返金済み）。辞退の記録だけを外す。
ALTER TABLE review_requests
    DROP CONSTRAINT review_requests_withdraw_check,
    DROP COLUMN withdrawn_at,
    DROP COLUMN withdrawn_by,
    DROP COLUMN withdrawn_from_status,
    DROP COLUMN withdraw_reason;
