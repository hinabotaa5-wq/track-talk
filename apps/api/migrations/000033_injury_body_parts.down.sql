UPDATE injury_details SET body_part = 'upper-body' WHERE body_part IN ('abs', 'scapula', 'shoulder', 'neck');

ALTER TABLE injury_details DROP CONSTRAINT injury_details_body_part_check;
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
