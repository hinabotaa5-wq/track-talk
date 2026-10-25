DELETE FROM reports WHERE target_type='voice_participant';
ALTER TABLE reports DROP CONSTRAINT reports_target_type_check;
ALTER TABLE reports ADD CONSTRAINT reports_target_type_check CHECK (target_type IN ('post','comment','injury_note','review_response','coaching_message','practice_rating','coaching_consultation'));
DROP TABLE meet_voice_actions;
DROP TABLE meet_voice_participants;
ALTER TABLE meet_voice_reservations DROP COLUMN solo_since, DROP COLUMN confirmed_at, DROP COLUMN last_seen_at;
DROP TABLE meet_voice_settings;
