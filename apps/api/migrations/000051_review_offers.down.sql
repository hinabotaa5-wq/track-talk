DELETE FROM notifications WHERE review_request_id IS NOT NULL OR kind IN ('review_offer', 'review_expired');
ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications
    ADD CONSTRAINT notifications_kind_check CHECK (kind IN (
        'recruitment_post',
        'comment',
        'comment_reply',
        'practice_application',
        'practice_accepted',
        'practice_rate',
        'injury_update'
    ));
ALTER TABLE notifications
    DROP CONSTRAINT notifications_target_check,
    DROP COLUMN review_request_id,
    ALTER COLUMN post_id SET NOT NULL;
DROP INDEX review_requests_coach_idx;
DROP TABLE review_offers;
