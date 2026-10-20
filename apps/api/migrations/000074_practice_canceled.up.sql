-- 練習の日時より前に解散した練習は中止として扱い、参加者を評価できないようにする。
-- 解散したあとに日時を書き換えられても変わらないよう、解散した時点で決めて残す。
ALTER TABLE practice_details ADD COLUMN canceled boolean NOT NULL DEFAULT false;
