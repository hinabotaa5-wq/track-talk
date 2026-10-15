ALTER TABLE injury_group_members
    DROP CONSTRAINT IF EXISTS injury_group_members_diagnosis_check,
    DROP CONSTRAINT IF EXISTS injury_group_members_severity_check,
    DROP CONSTRAINT IF EXISTS injury_group_members_place_kind_check,
    DROP CONSTRAINT IF EXISTS injury_group_members_activity_check,
    DROP CONSTRAINT IF EXISTS injury_group_members_pre_changes_check,
    DROP CONSTRAINT IF EXISTS injury_group_members_event_check,
    DROP CONSTRAINT IF EXISTS injury_group_members_place_note_check,
    DROP CONSTRAINT IF EXISTS injury_group_members_situation_note_check,
    DROP COLUMN diagnosis,
    DROP COLUMN severity,
    DROP COLUMN saw_doctor,
    DROP COLUMN place,
    DROP COLUMN activity,
    DROP COLUMN pre_changes,
    DROP COLUMN event;
-- 選択式で入った人には自由記述がないので、戻すときは空の代わりに「不明」を入れる。
UPDATE injury_group_members SET place_note = COALESCE(place_note, '不明'), situation_note = COALESCE(situation_note, '不明');
ALTER TABLE injury_group_members RENAME COLUMN place_note TO place;
ALTER TABLE injury_group_members RENAME COLUMN situation_note TO situation;
ALTER TABLE injury_group_members
    ALTER COLUMN place SET NOT NULL,
    ALTER COLUMN situation SET NOT NULL,
    ADD CONSTRAINT injury_group_members_place_check CHECK (char_length(btrim(place)) BETWEEN 1 AND 100),
    ADD CONSTRAINT injury_group_members_situation_check CHECK (char_length(btrim(situation)) BETWEEN 1 AND 500);

-- 診断名別のグループを、近い部位へ戻す。対応する部位がないものは「その他」にする。
ALTER TABLE injury_details DROP CONSTRAINT IF EXISTS injury_details_body_part_check;
ALTER TABLE injury_group_members DROP CONSTRAINT IF EXISTS injury_group_members_body_part_check;
DELETE FROM injury_group_members m
WHERE m.body_part = 'elbow'
  AND EXISTS (SELECT 1 FROM injury_group_members o WHERE o.user_id = m.user_id AND o.body_part = 'other');
ALTER TABLE injury_group_stage_log DROP CONSTRAINT IF EXISTS injury_group_stage_log_user_id_body_part_fkey;
CREATE TABLE injury_group_migration_map (old_part text PRIMARY KEY, new_part text NOT NULL);
INSERT INTO injury_group_migration_map (old_part, new_part) VALUES
    ('hamstring-strain', 'hamstring'),
    ('quad-strain', 'quad'),
    ('sprain', 'ankle'),
    ('achilles-tendinitis', 'achilles'),
    ('jumpers-knee', 'knee'),
    ('shin-splints', 'shin'),
    ('shoulder', 'shoulder');
UPDATE injury_group_members g
SET body_part = COALESCE((SELECT mm.new_part FROM injury_group_migration_map mm WHERE mm.old_part = g.body_part), 'other');
UPDATE injury_group_stage_log l
SET body_part = COALESCE((SELECT mm.new_part FROM injury_group_migration_map mm WHERE mm.old_part = l.body_part), 'other');
UPDATE injury_details d
SET body_part = COALESCE((SELECT mm.new_part FROM injury_group_migration_map mm WHERE mm.old_part = d.body_part), 'other');
DROP TABLE injury_group_migration_map;
ALTER TABLE injury_group_stage_log ADD CONSTRAINT injury_group_stage_log_user_id_body_part_fkey
    FOREIGN KEY (user_id, body_part) REFERENCES injury_group_members (user_id, body_part) ON DELETE CASCADE;
ALTER TABLE injury_details ADD CONSTRAINT injury_details_body_part_check CHECK (
    body_part IN ('foot', 'ankle', 'achilles', 'calf', 'shin', 'knee', 'quad', 'hamstring', 'groin', 'hip', 'lower-back', 'abs', 'scapula', 'shoulder', 'neck', 'other')
);
ALTER TABLE injury_group_members ADD CONSTRAINT injury_group_members_body_part_check CHECK (
    body_part IN ('foot', 'ankle', 'achilles', 'calf', 'shin', 'knee', 'quad', 'hamstring', 'groin', 'hip', 'lower-back', 'abs', 'scapula', 'shoulder', 'neck', 'other')
);
