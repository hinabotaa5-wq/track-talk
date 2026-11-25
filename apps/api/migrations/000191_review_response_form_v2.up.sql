-- スタンダードの回答の新しい型と、申込の任意入力（本体 #1013、補足 2026-11-25）。
-- 回答：一言診断（既存 hitokoto）→良い点→ボトルネック（既存 fix_points）→キュー→ドリル→成功の目安→次に撮る動画。
-- 新規3欄（cue／success_criteria／next_video）は NULL 可で追加し、過去の返答は NULL のまま残して表示側で節ごと隠す。
ALTER TABLE review_responses
    ADD COLUMN cue text,
    ADD COLUMN success_criteria text,
    ADD COLUMN next_video text;

ALTER TABLE review_responses
    ADD CONSTRAINT review_responses_cue_length CHECK (cue IS NULL OR char_length(cue) BETWEEN 1 AND 300),
    ADD CONSTRAINT review_responses_success_criteria_length CHECK (success_criteria IS NULL OR char_length(success_criteria) BETWEEN 1 AND 300),
    ADD CONSTRAINT review_responses_next_video_length CHECK (next_video IS NULL OR char_length(next_video) BETWEEN 1 AND 300);

-- 申込で選手が任意で書ける選手情報（PB・目標タイム・風速・その時の体のコンディション）。
-- 未入力は NULL。風速・コンディションも自由記述のテキストとして持つ（例：「+1.2」「向かい風で強い」）。
ALTER TABLE review_requests
    ADD COLUMN pb text,
    ADD COLUMN goal_time text,
    ADD COLUMN wind_speed text,
    ADD COLUMN body_condition text;

ALTER TABLE review_requests
    ADD CONSTRAINT review_requests_pb_length CHECK (pb IS NULL OR char_length(pb) BETWEEN 1 AND 100),
    ADD CONSTRAINT review_requests_goal_time_length CHECK (goal_time IS NULL OR char_length(goal_time) BETWEEN 1 AND 100),
    ADD CONSTRAINT review_requests_wind_speed_length CHECK (wind_speed IS NULL OR char_length(wind_speed) BETWEEN 1 AND 50),
    ADD CONSTRAINT review_requests_body_condition_length CHECK (body_condition IS NULL OR char_length(body_condition) BETWEEN 1 AND 300);
