DELETE FROM notification_settings WHERE category = 'announcement';
ALTER TABLE notification_settings DROP CONSTRAINT notification_settings_category_check;
ALTER TABLE notification_settings
    ADD CONSTRAINT notification_settings_category_check CHECK (category IN (
        'comment',
        'practice',
        'recruitment',
        'injury',
        'review',
        'coaching',
        'monthly'
    ));

DROP INDEX IF EXISTS notifications_announcement_idx;
ALTER TABLE notifications DROP COLUMN announcement_id;
DROP TABLE announcements;
