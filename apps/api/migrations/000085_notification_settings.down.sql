DELETE FROM notifications WHERE kind = 'message';
ALTER TABLE notifications DROP CONSTRAINT notifications_message_check;
ALTER TABLE notifications DROP CONSTRAINT notifications_target_check;
ALTER TABLE notifications
    ADD CONSTRAINT notifications_target_check CHECK (post_id IS NOT NULL OR review_request_id IS NOT NULL);
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
        'review_helpful_ask'
    ));
ALTER TABLE notifications
    DROP COLUMN title,
    DROP COLUMN body,
    DROP COLUMN url;
DROP TABLE IF EXISTS notification_settings;
