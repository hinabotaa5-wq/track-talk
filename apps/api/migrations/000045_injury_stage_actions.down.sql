ALTER TABLE injury_group_stage_log
    DROP CONSTRAINT IF EXISTS injury_group_stage_log_actions_check,
    DROP CONSTRAINT IF EXISTS injury_group_stage_log_note_check,
    DROP CONSTRAINT IF EXISTS injury_group_stage_log_note_public_check,
    DROP COLUMN actions,
    DROP COLUMN note,
    DROP COLUMN note_public;
