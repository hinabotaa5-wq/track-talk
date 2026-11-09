ALTER TABLE coaching_consultations DROP COLUMN videos_purged_at;
ALTER TABLE coaching_contracts DROP COLUMN close_notice_at, DROP COLUMN retention_hold_at, DROP COLUMN videos_purged_at;
ALTER TABLE review_requests DROP COLUMN retention_hold_at, DROP COLUMN videos_purged_at;
