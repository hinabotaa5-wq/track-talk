DELETE FROM notifications WHERE kind IN ('review_offer_reminder', 'review_accepted', 'review_nomination_ended');
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
        'review_withdrawn',
        'message'
    ));

ALTER TABLE review_offers DROP COLUMN reminder_sent_at;
