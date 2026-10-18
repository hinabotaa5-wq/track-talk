DELETE FROM reports WHERE target_type = 'practice_rating';
ALTER TABLE reports
    DROP CONSTRAINT reports_target_type_check,
    ADD CONSTRAINT reports_target_type_check CHECK (target_type IN ('post', 'comment', 'injury_note', 'review_response', 'coaching_message'));

ALTER TABLE practice_ratings DROP COLUMN hidden_at;
