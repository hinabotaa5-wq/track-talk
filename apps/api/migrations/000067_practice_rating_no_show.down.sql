-- 制約を戻す前に 'no_show' の評価を消す(残っていると CHECK 違反になる)。
DELETE FROM practice_ratings WHERE score = 'no_show';

ALTER TABLE practice_ratings DROP CONSTRAINT practice_ratings_score_check;
ALTER TABLE practice_ratings
    ADD CONSTRAINT practice_ratings_score_check CHECK (score IN ('good', 'ok', 'bad'));
