-- post_like の通知(と未処理の通知予定)を消してから、000165 以前の列挙に戻す。
DELETE FROM notifications WHERE kind = 'post_like';
DELETE FROM notification_jobs WHERE kind = 'post_like';
DO $$
DECLARE
    original_comment text;
BEGIN
    SELECT obj_description(oid, 'pg_constraint') INTO STRICT original_comment
    FROM pg_constraint
    WHERE conrelid = 'notifications'::regclass AND conname = 'notifications_kind_check';

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
            'review_offer_reminder',
            'review_accepted',
            'review_nomination_ended',
            'message',
            'review_resubmitted_after_accept'
        ));
    IF original_comment IS NOT NULL THEN
        EXECUTE format('COMMENT ON CONSTRAINT notifications_kind_check ON notifications IS %L', original_comment);
    END IF;
END $$;
