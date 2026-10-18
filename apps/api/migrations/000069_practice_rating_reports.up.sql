-- 練習の評価のひとことは、練習の投稿でログインなしに誰でも読める。
-- 通報できるようにし、運営が非表示にできるようにする。
ALTER TABLE practice_ratings ADD COLUMN hidden_at timestamptz;

ALTER TABLE reports
    DROP CONSTRAINT reports_target_type_check,
    ADD CONSTRAINT reports_target_type_check CHECK (target_type IN ('post', 'comment', 'injury_note', 'review_response', 'coaching_message', 'practice_rating'));
