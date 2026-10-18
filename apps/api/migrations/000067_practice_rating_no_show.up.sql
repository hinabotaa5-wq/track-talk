-- 主催者が参加者を評価するときに「ドタキャン」を選べるようにする。
ALTER TABLE practice_ratings DROP CONSTRAINT practice_ratings_score_check;
ALTER TABLE practice_ratings
    ADD CONSTRAINT practice_ratings_score_check CHECK (score IN ('good', 'ok', 'bad', 'no_show'));
