DELETE FROM reports WHERE target_type = 'review_response';
ALTER TABLE reports DROP CONSTRAINT reports_target_type_check;
ALTER TABLE reports ADD CONSTRAINT reports_target_type_check CHECK (target_type IN ('post', 'comment', 'injury_note'));
DELETE FROM notifications WHERE kind IN ('review_returned', 'review_clarify', 'review_clarified', 'review_helpful_ask');
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
        'review_expired'
    ));
DROP TABLE review_ratings;
DROP TABLE review_responses;
ALTER TABLE review_requests DROP COLUMN returned_at;
