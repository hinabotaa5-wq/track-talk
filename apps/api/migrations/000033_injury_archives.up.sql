-- 退会に同意した人のケガの記録を、ユーザーと結びつかない数字と選択式だけにして残す。
-- user_id や post_id は持たない。日付は月までに丸める。自由記述(場所・状況・投稿・経過の文章)は残さない。
CREATE TABLE injury_archives (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    body_part text NOT NULL,
    injured_month date NOT NULL,
    archived_month date NOT NULL,
    -- 段階の移り変わり。days は最初の段階からの日数。例: [{"stage":"resting","days":0},{"stage":"rehab","days":12}]
    stage_timeline jsonb NOT NULL DEFAULT '[]'
);

CREATE INDEX injury_archives_part_idx ON injury_archives (body_part);

ALTER TABLE injury_archives ENABLE ROW LEVEL SECURITY;
