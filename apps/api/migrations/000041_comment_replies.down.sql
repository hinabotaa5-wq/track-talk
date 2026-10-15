DELETE FROM notifications WHERE kind = 'comment_reply';
ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications
    ADD CONSTRAINT notifications_kind_check CHECK (kind IN (
        'recruitment_post',
        'comment',
        'practice_application',
        'practice_accepted',
        'practice_rate',
        'injury_update'
    ));
DROP INDEX comments_parent_idx;
ALTER TABLE comments DROP COLUMN reply_to_id;
