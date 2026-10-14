DELETE FROM injury_group_members m
WHERE m.body_part IN ('abs', 'scapula', 'shoulder', 'neck')
  AND EXISTS (SELECT 1 FROM injury_group_members o WHERE o.user_id = m.user_id AND o.body_part = 'upper-body');

ALTER TABLE injury_group_stage_log DROP CONSTRAINT IF EXISTS injury_group_stage_log_user_id_body_part_fkey;
UPDATE injury_group_members SET body_part = 'upper-body' WHERE body_part IN ('abs', 'scapula', 'shoulder', 'neck');
UPDATE injury_group_stage_log SET body_part = 'upper-body' WHERE body_part IN ('abs', 'scapula', 'shoulder', 'neck');
ALTER TABLE injury_group_stage_log ADD CONSTRAINT injury_group_stage_log_user_id_body_part_fkey
    FOREIGN KEY (user_id, body_part) REFERENCES injury_group_members (user_id, body_part) ON DELETE CASCADE;
UPDATE injury_details SET body_part = 'upper-body' WHERE body_part IN ('abs', 'scapula', 'shoulder', 'neck');

ALTER TABLE injury_details DROP CONSTRAINT IF EXISTS injury_details_body_part_check;
ALTER TABLE injury_details ADD CONSTRAINT injury_details_body_part_check CHECK (
    body_part IN (
        'foot',
        'ankle',
        'achilles',
        'calf',
        'shin',
        'knee',
        'quad',
        'hamstring',
        'groin',
        'hip',
        'lower-back',
        'upper-body',
        'other'
    )
);

ALTER TABLE injury_group_members DROP CONSTRAINT IF EXISTS injury_group_members_body_part_check;
ALTER TABLE injury_group_members ADD CONSTRAINT injury_group_members_body_part_check CHECK (
    body_part IN (
        'foot',
        'ankle',
        'achilles',
        'calf',
        'shin',
        'knee',
        'quad',
        'hamstring',
        'groin',
        'hip',
        'lower-back',
        'upper-body',
        'other'
    )
);
