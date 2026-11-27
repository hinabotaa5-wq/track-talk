-- 注意：この down は、プレミアムの依頼（動画・総括を含む）を消す。消した行は戻せない。
-- 動画ファイル（Cloudinary）は消さないので、戻す前に保持期限での削除が済んでいることを確かめること。
DROP TABLE review_response_sections;

DELETE FROM review_responses WHERE good_point IS NULL;

ALTER TABLE review_responses
    DROP CONSTRAINT review_responses_standard_fields;

ALTER TABLE review_responses
    ALTER COLUMN good_point SET NOT NULL,
    ALTER COLUMN fix_points SET NOT NULL,
    ALTER COLUMN drill SET NOT NULL;

DROP TABLE review_request_videos;

DELETE FROM review_requests WHERE plan = 'premium';

ALTER TABLE review_requests
    DROP CONSTRAINT review_requests_question_or_focus_check;

ALTER TABLE review_requests
    ADD CONSTRAINT review_requests_question_or_focus_check CHECK (char_length(btrim(question)) > 0 OR cardinality(focus) > 0);

ALTER TABLE review_requests
    DROP CONSTRAINT review_requests_plan_check,
    DROP COLUMN plan;
