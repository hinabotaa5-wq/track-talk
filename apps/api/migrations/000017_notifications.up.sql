CREATE TABLE notifications (
    id uuid PRIMARY KEY,
    recipient_id uuid NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN (
        'recruitment_post',
        'comment',
        'practice_application',
        'practice_accepted',
        'practice_rate'
    )),
    post_id uuid NOT NULL REFERENCES posts (id) ON DELETE CASCADE,
    comment_id uuid REFERENCES comments (id) ON DELETE CASCADE,
    application_id uuid REFERENCES practice_applications (id) ON DELETE CASCADE,
    read_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX notifications_recipient_created_idx
    ON notifications (recipient_id, created_at DESC);

CREATE INDEX notifications_recipient_unread_idx
    ON notifications (recipient_id)
    WHERE read_at IS NULL;

ALTER TABLE notifications ENABLE ROW LEVEL SECURITY;
