ALTER TABLE profiles
    DROP COLUMN push_ever_subscribed;

ALTER TABLE monthly_reminders
    DROP COLUMN in_app_done_at;

DROP TABLE push_outbox;
