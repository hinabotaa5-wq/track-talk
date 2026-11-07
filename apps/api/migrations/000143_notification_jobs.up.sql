CREATE TABLE notification_jobs (
    id uuid PRIMARY KEY,
    kind text NOT NULL,
    recipient_id uuid NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    post_id uuid REFERENCES posts (id) ON DELETE CASCADE,
    comment_id uuid REFERENCES comments (id) ON DELETE CASCADE,
    application_id uuid REFERENCES practice_applications (id) ON DELETE CASCADE,
    review_request_id uuid REFERENCES review_requests (id) ON DELETE CASCADE,
    reviewed_at timestamptz,
    next_attempt_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE notification_jobs ENABLE ROW LEVEL SECURITY;
CREATE INDEX notification_jobs_due_idx ON notification_jobs (next_attempt_at, id);
CREATE INDEX notification_jobs_rejection_idx ON notification_jobs (recipient_id, reviewed_at)
    WHERE kind = 'coach_rejected';

-- 並行作業で追加された通知種別も保持する。
DO $$
DECLARE
    existing_check text;
BEGIN
    SELECT pg_get_expr(conbin, conrelid) INTO STRICT existing_check
    FROM pg_constraint
    WHERE conrelid = 'notifications'::regclass AND conname = 'notifications_kind_check';

    ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
    EXECUTE format(
        'ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check CHECK ((%s) OR kind = %L)',
        existing_check, 'review_resubmitted_after_accept'
    );
    EXECUTE format(
        'COMMENT ON CONSTRAINT notifications_kind_check ON notifications IS %L',
        existing_check
    );
END $$;
