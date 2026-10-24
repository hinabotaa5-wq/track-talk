ALTER TABLE review_requests ADD COLUMN question text NOT NULL DEFAULT '' CHECK (char_length(question) <= 1000);
ALTER TABLE review_requests DROP CONSTRAINT review_requests_focus_check;
ALTER TABLE review_requests ADD CONSTRAINT review_requests_focus_check CHECK (
    cardinality(focus) BETWEEN 0 AND 8
    AND focus <@ ARRAY['start', 'acceleration', 'top-speed', 'takeoff', 'arm', 'contact', 'rhythm', 'other']::text[]
);
ALTER TABLE review_requests ADD CONSTRAINT review_requests_question_or_focus_check CHECK (char_length(btrim(question)) > 0 OR cardinality(focus) > 0);
