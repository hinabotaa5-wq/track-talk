DROP INDEX injury_details_mood_idx;
ALTER TABLE injury_details DROP COLUMN mood;
CREATE INDEX injury_details_body_part_idx ON injury_details (body_part);
