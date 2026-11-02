DROP INDEX IF EXISTS practice_details_upcoming_idx;
ALTER TABLE practice_details
    DROP CONSTRAINT IF EXISTS practice_details_prefecture_check,
    DROP COLUMN IF EXISTS prefecture;
