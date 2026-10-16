-- ケガ部のグループを、部位別の16個から診断名別の8個 + その他に置き換える。
-- 診断名がはっきり対応する部位だけ移し、それ以外(膝を含む)は「その他」にまとめる。
CREATE TABLE injury_group_migration_map (old_part text PRIMARY KEY, new_part text NOT NULL);
INSERT INTO injury_group_migration_map (old_part, new_part) VALUES
    ('hamstring', 'hamstring-strain'),
    ('quad', 'quad-strain'),
    ('ankle', 'sprain'),
    ('achilles', 'achilles-tendinitis'),
    ('shin', 'shin-splints'),
    ('shoulder', 'shoulder');

ALTER TABLE injury_details DROP CONSTRAINT IF EXISTS injury_details_body_part_check;
ALTER TABLE injury_group_members DROP CONSTRAINT IF EXISTS injury_group_members_body_part_check;

-- 同じ人の複数のグループが「その他」にまとまるときは、いちばん最近更新したものだけ残す。
DELETE FROM injury_group_members m
USING (
    SELECT g.user_id, g.body_part,
           row_number() OVER (
               PARTITION BY g.user_id, COALESCE(mm.new_part, 'other')
               ORDER BY g.updated_at DESC
           ) AS rn
    FROM injury_group_members g
    LEFT JOIN injury_group_migration_map mm ON mm.old_part = g.body_part
) ranked
WHERE m.user_id = ranked.user_id
  AND m.body_part = ranked.body_part
  AND ranked.rn > 1;

ALTER TABLE injury_group_stage_log DROP CONSTRAINT IF EXISTS injury_group_stage_log_user_id_body_part_fkey;
UPDATE injury_group_members g
SET body_part = COALESCE((SELECT mm.new_part FROM injury_group_migration_map mm WHERE mm.old_part = g.body_part), 'other');
UPDATE injury_group_stage_log l
SET body_part = COALESCE((SELECT mm.new_part FROM injury_group_migration_map mm WHERE mm.old_part = l.body_part), 'other');
UPDATE injury_details d
SET body_part = COALESCE((SELECT mm.new_part FROM injury_group_migration_map mm WHERE mm.old_part = d.body_part), 'other');
ALTER TABLE injury_group_stage_log ADD CONSTRAINT injury_group_stage_log_user_id_body_part_fkey
    FOREIGN KEY (user_id, body_part) REFERENCES injury_group_members (user_id, body_part) ON DELETE CASCADE;

DROP TABLE injury_group_migration_map;

ALTER TABLE injury_details ADD CONSTRAINT injury_details_body_part_check CHECK (
    body_part IN ('hamstring-strain', 'quad-strain', 'sprain', 'achilles-tendinitis', 'jumpers-knee', 'shin-splints', 'shoulder', 'elbow', 'other')
);
ALTER TABLE injury_group_members ADD CONSTRAINT injury_group_members_body_part_check CHECK (
    body_part IN ('hamstring-strain', 'quad-strain', 'sprain', 'achilles-tendinitis', 'jumpers-knee', 'shin-splints', 'shoulder', 'elbow', 'other')
);

-- 入るときの質問を、予防と復帰の目安に集計できる選択式にする。
-- 前の自由記述(場所・状況)は消さずに残し、新しく入る人には聞かない。
ALTER TABLE injury_group_members DROP CONSTRAINT IF EXISTS injury_group_members_place_check;
ALTER TABLE injury_group_members DROP CONSTRAINT IF EXISTS injury_group_members_situation_check;
ALTER TABLE injury_group_members RENAME COLUMN place TO place_note;
ALTER TABLE injury_group_members RENAME COLUMN situation TO situation_note;
ALTER TABLE injury_group_members
    ALTER COLUMN place_note DROP NOT NULL,
    ALTER COLUMN situation_note DROP NOT NULL,
    ADD CONSTRAINT injury_group_members_place_note_check CHECK (place_note IS NULL OR char_length(btrim(place_note)) BETWEEN 1 AND 100),
    ADD CONSTRAINT injury_group_members_situation_note_check CHECK (situation_note IS NULL OR char_length(btrim(situation_note)) BETWEEN 1 AND 500),
    ADD COLUMN diagnosis text,
    ADD COLUMN severity text,
    ADD COLUMN saw_doctor boolean,
    ADD COLUMN place text,
    ADD COLUMN activity text,
    ADD COLUMN pre_changes text[] NOT NULL DEFAULT '{}',
    ADD COLUMN event text,
    ADD CONSTRAINT injury_group_members_diagnosis_check CHECK (diagnosis IS NULL OR char_length(btrim(diagnosis)) BETWEEN 1 AND 40),
    ADD CONSTRAINT injury_group_members_severity_check CHECK (severity IS NULL OR severity IN ('mild', 'moderate', 'severe')),
    ADD CONSTRAINT injury_group_members_place_kind_check CHECK (
        place IS NULL OR place IN ('tartan', 'grass', 'dirt', 'asphalt', 'slope', 'stairs', 'weight-room', 'gym')
    ),
    ADD CONSTRAINT injury_group_members_activity_check CHECK (
        activity IS NULL OR activity IN ('jog', 'start', 'sprint', 'jump-hurdle', 'throw', 'weights', 'outside')
    ),
    ADD CONSTRAINT injury_group_members_pre_changes_check CHECK (
        pre_changes <@ ARRAY['more-volume', 'new-shoes', 'comeback', 'fatigue', 'played-through-pain', 'none']::text[]
    ),
    ADD CONSTRAINT injury_group_members_event_check CHECK (
        event IS NULL OR event IN ('sprint', 'hurdles', 'middle', 'long', 'jumps', 'throws', 'combined', 'race-walk')
    );
