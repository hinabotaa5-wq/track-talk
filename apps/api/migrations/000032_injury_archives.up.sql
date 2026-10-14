-- 退会に同意した人のケガの記録を、ユーザーと結びつかない形で残す。
-- user_id や post_id は持たない。日付は月までに丸め、自由記述は個人が特定できそうな記述を取り除いてから入れる。
CREATE TABLE injury_archives (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    body_part text NOT NULL,
    injured_month date NOT NULL,
    archived_month date NOT NULL,
    stage_timeline jsonb NOT NULL DEFAULT '[]',
    place text NOT NULL DEFAULT '',
    situation text NOT NULL DEFAULT '',
    posts jsonb NOT NULL DEFAULT '[]',
    -- 自由記述に個人が特定できる記述が残っていないか、人の目で確かめるまでは pending。分析に使うのは approved だけ。
    review_status text NOT NULL DEFAULT 'pending',
    CHECK (review_status IN ('pending', 'approved', 'rejected'))
);

CREATE INDEX injury_archives_part_idx ON injury_archives (body_part);

ALTER TABLE injury_archives ENABLE ROW LEVEL SECURITY;
