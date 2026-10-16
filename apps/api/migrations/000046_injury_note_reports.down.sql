DELETE FROM reports WHERE target_type = 'injury_note';
ALTER TABLE reports DROP CONSTRAINT reports_target_type_check;
ALTER TABLE reports ADD CONSTRAINT reports_target_type_check CHECK (target_type IN ('post', 'comment'));

ALTER TABLE injury_group_stage_log
    DROP CONSTRAINT injury_group_stage_log_note_id_key,
    DROP COLUMN hidden_at,
    DROP COLUMN note_id;
