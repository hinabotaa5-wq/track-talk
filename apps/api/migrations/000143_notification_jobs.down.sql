DROP TABLE notification_jobs;

UPDATE notifications SET kind = 'review_resubmitted', url = NULL
WHERE kind = 'review_resubmitted_after_accept';
DO $$
DECLARE
    original_check text;
BEGIN
    SELECT obj_description(oid, 'pg_constraint') INTO STRICT original_check
    FROM pg_constraint
    WHERE conrelid = 'notifications'::regclass AND conname = 'notifications_kind_check';

    ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
    EXECUTE format(
        'ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check CHECK (%s)',
        original_check
    );
END $$;
