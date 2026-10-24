ALTER TABLE review_requests DROP CONSTRAINT review_requests_question_or_focus_check;
ALTER TABLE review_requests DROP CONSTRAINT review_requests_focus_check;
UPDATE review_requests SET focus=ARRAY['other']::text[] WHERE cardinality(focus)=0;
ALTER TABLE review_requests ADD CONSTRAINT review_requests_focus_check CHECK (
    cardinality(focus) BETWEEN 1 AND 8
    AND focus <@ ARRAY['start', 'acceleration', 'top-speed', 'takeoff', 'arm', 'contact', 'rhythm', 'other']::text[]
);
ALTER TABLE review_requests DROP COLUMN question;
