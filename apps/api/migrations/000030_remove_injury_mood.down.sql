DROP INDEX injury_details_body_part_idx;
ALTER TABLE injury_details ADD COLUMN mood text NOT NULL DEFAULT 'vent' CHECK (mood IN ('vent', 'rehab'));
ALTER TABLE injury_details ALTER COLUMN mood DROP DEFAULT;
CREATE INDEX injury_details_mood_idx ON injury_details (mood, body_part);
