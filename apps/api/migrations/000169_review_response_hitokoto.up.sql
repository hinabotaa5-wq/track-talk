-- コーチの返答への「ひとこと」欄。良い点1つ・直す点1つのあとに添える短いまとめ。
-- 過去の返答は NULL のまま残し、表示側で隠す。
ALTER TABLE review_responses
    ADD COLUMN hitokoto text;

ALTER TABLE review_responses
    ADD CONSTRAINT review_responses_hitokoto_length CHECK (hitokoto IS NULL OR char_length(hitokoto) BETWEEN 1 AND 300);
