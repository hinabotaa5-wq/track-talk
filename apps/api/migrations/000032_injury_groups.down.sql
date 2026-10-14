ALTER TABLE injury_details
    DROP CONSTRAINT injury_details_group_stage_check,
    DROP COLUMN group_stage;
DROP TABLE injury_group_stage_log;
DROP TABLE injury_group_members;
