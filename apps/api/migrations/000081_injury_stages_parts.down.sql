ALTER TABLE injury_group_members DROP COLUMN injured_on_exact;

ALTER TABLE injury_updates DROP CONSTRAINT injury_updates_stage_check;
ALTER TABLE injury_group_members DROP CONSTRAINT injury_group_members_stage_check;
ALTER TABLE injury_group_stage_log DROP CONSTRAINT injury_group_stage_log_stage_check;
ALTER TABLE injury_details DROP CONSTRAINT injury_details_group_stage_check;

UPDATE injury_updates SET stage = 'practice' WHERE stage IN ('half', 'full');
UPDATE injury_group_members SET stage = 'practice' WHERE stage IN ('half', 'full');
UPDATE injury_details SET group_stage = 'practice' WHERE group_stage IN ('half', 'full');
UPDATE injury_group_stage_log SET stage = 'practice' WHERE stage IN ('half', 'full');

ALTER TABLE injury_updates ADD CONSTRAINT injury_updates_stage_check CHECK (stage IN ('resting', 'rehab', 'jogging', 'practice', 'race'));
ALTER TABLE injury_group_members ADD CONSTRAINT injury_group_members_stage_check CHECK (stage IN ('resting', 'rehab', 'jogging', 'practice', 'race'));
ALTER TABLE injury_group_stage_log ADD CONSTRAINT injury_group_stage_log_stage_check CHECK (stage IN ('resting', 'rehab', 'jogging', 'practice', 'race'));
ALTER TABLE injury_details ADD CONSTRAINT injury_details_group_stage_check CHECK (group_stage IS NULL OR group_stage IN ('resting', 'rehab', 'jogging', 'practice', 'race'));

ALTER TABLE injury_details DROP CONSTRAINT injury_details_body_part_check;
ALTER TABLE injury_group_members DROP CONSTRAINT injury_group_members_body_part_check;
DELETE FROM injury_group_members WHERE body_part IN ('stress-fracture', 'calf-strain', 'lower-back');
UPDATE injury_details SET body_part = 'other' WHERE body_part IN ('stress-fracture', 'calf-strain', 'lower-back');
ALTER TABLE injury_details ADD CONSTRAINT injury_details_body_part_check CHECK (
    body_part IN ('hamstring-strain', 'quad-strain', 'sprain', 'achilles-tendinitis', 'jumpers-knee', 'shin-splints', 'shoulder', 'elbow', 'other')
);
ALTER TABLE injury_group_members ADD CONSTRAINT injury_group_members_body_part_check CHECK (
    body_part IN ('hamstring-strain', 'quad-strain', 'sprain', 'achilles-tendinitis', 'jumpers-knee', 'shin-splints', 'shoulder', 'elbow', 'other')
);
