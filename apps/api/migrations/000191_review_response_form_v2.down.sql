-- 注意：この down は、000191 で追加した列を消す。新しい型で発行済みの返答（キュー・成功の目安・次に撮る動画）と、
-- 申込の任意入力（PB・目標タイム・風速・その時の体のコンディション）に値がある行があると、その値は失われ、戻せない。
ALTER TABLE review_requests
    DROP CONSTRAINT review_requests_body_condition_length,
    DROP CONSTRAINT review_requests_wind_speed_length,
    DROP CONSTRAINT review_requests_goal_time_length,
    DROP CONSTRAINT review_requests_pb_length;

ALTER TABLE review_requests
    DROP COLUMN body_condition,
    DROP COLUMN wind_speed,
    DROP COLUMN goal_time,
    DROP COLUMN pb;

ALTER TABLE review_responses
    DROP CONSTRAINT review_responses_next_video_length,
    DROP CONSTRAINT review_responses_success_criteria_length,
    DROP CONSTRAINT review_responses_cue_length;

ALTER TABLE review_responses
    DROP COLUMN next_video,
    DROP COLUMN success_criteria,
    DROP COLUMN cue;
