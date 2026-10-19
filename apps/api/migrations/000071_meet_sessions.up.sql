-- 主要大会の種目ごとの開始時刻(タイムテーブル)。運営が管理画面から CSV を貼り付けて入れる。
-- 「いま実施中」は、現在時刻が starts_at から次の開始までのあいだにある種目。
-- meet_key は meet_threads と同じ識別子。competitions へは外部キーを張らない(取り込みで行が作り直されるため)。
CREATE TABLE meet_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    meet_key text NOT NULL,
    event text NOT NULL CHECK (char_length(event) BETWEEN 1 AND 30),
    round text NOT NULL DEFAULT '' CHECK (char_length(round) <= 20),
    gender text NOT NULL DEFAULT '' CHECK (gender IN ('', '男子', '女子', '混合')),
    starts_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX meet_sessions_meet_key_starts_at_idx ON meet_sessions (meet_key, starts_at);

ALTER TABLE meet_sessions ENABLE ROW LEVEL SECURITY;
