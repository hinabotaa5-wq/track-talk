ALTER TABLE review_responses
    DROP CONSTRAINT review_responses_hitokoto_length;

ALTER TABLE review_responses
    DROP COLUMN hitokoto;
