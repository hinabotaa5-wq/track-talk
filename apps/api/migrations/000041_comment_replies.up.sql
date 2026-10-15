ALTER TABLE comments
    ADD COLUMN reply_to_id uuid REFERENCES comments (id);

CREATE INDEX comments_parent_idx ON comments (parent_id) WHERE parent_id IS NOT NULL;

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
