-- ケガ部の見直し。
-- 1. ケガの種類を12個にする(足首の捻挫・疲労骨折・ふくらはぎの肉離れ・腰の痛みを追加。捻挫は「足首の捻挫」に名前を変える)。
-- 2. 段階を6つにする(「練習に復帰」を「50%を出せる」と「全力を出せる」に分ける)。すでに「練習に復帰」の人は「50%を出せる」に移す。
-- 3. いつケガをしたかを日まで聞く。日まで答えた人は injured_on_exact を true にして、月だけ答えた以前の人と区別する。

ALTER TABLE injury_details DROP CONSTRAINT injury_details_body_part_check;
ALTER TABLE injury_group_members DROP CONSTRAINT injury_group_members_body_part_check;
ALTER TABLE injury_details ADD CONSTRAINT injury_details_body_part_check CHECK (
    body_part IN ('hamstring-strain', 'sprain', 'shin-splints', 'stress-fracture', 'calf-strain', 'jumpers-knee', 'achilles-tendinitis', 'lower-back', 'quad-strain', 'shoulder', 'elbow', 'other')
);
ALTER TABLE injury_group_members ADD CONSTRAINT injury_group_members_body_part_check CHECK (
    body_part IN ('hamstring-strain', 'sprain', 'shin-splints', 'stress-fracture', 'calf-strain', 'jumpers-knee', 'achilles-tendinitis', 'lower-back', 'quad-strain', 'shoulder', 'elbow', 'other')
);

ALTER TABLE injury_updates DROP CONSTRAINT injury_updates_stage_check;
ALTER TABLE injury_group_members DROP CONSTRAINT injury_group_members_stage_check;
ALTER TABLE injury_group_stage_log DROP CONSTRAINT injury_group_stage_log_stage_check;
ALTER TABLE injury_details DROP CONSTRAINT injury_details_group_stage_check;

UPDATE injury_updates SET stage = 'half' WHERE stage = 'practice';
UPDATE injury_group_members SET stage = 'half' WHERE stage = 'practice';
UPDATE injury_group_stage_log SET stage = 'half' WHERE stage = 'practice';
UPDATE injury_details SET group_stage = 'half' WHERE group_stage = 'practice';

ALTER TABLE injury_updates ADD CONSTRAINT injury_updates_stage_check CHECK (stage IN ('resting', 'rehab', 'jogging', 'half', 'full', 'race'));
ALTER TABLE injury_group_members ADD CONSTRAINT injury_group_members_stage_check CHECK (stage IN ('resting', 'rehab', 'jogging', 'half', 'full', 'race'));
ALTER TABLE injury_group_stage_log ADD CONSTRAINT injury_group_stage_log_stage_check CHECK (stage IN ('resting', 'rehab', 'jogging', 'half', 'full', 'race'));
ALTER TABLE injury_details ADD CONSTRAINT injury_details_group_stage_check CHECK (group_stage IS NULL OR group_stage IN ('resting', 'rehab', 'jogging', 'half', 'full', 'race'));

ALTER TABLE injury_group_members ADD COLUMN injured_on_exact boolean NOT NULL DEFAULT false;
