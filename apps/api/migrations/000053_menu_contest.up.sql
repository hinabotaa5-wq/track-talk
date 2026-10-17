-- エグ練大会。11月〜3月に2週間ごとに開き、その回で一番エグかった練習を1人1件だけ応募する。
-- 応募は運営だけが読み、運営が選んだトップ3(finalists)をみんなで投票して1位を決める。
-- 回は開始日(日本時間の月曜)で表す。日程は開始日から計算するので、回そのもののテーブルは持たない。
CREATE TABLE menu_contest_entries (
    id uuid PRIMARY KEY,
    round_start date NOT NULL,
    author_id uuid NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    body text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (char_length(btrim(body)) BETWEEN 1 AND 200),
    UNIQUE (round_start, author_id)
);

CREATE TABLE menu_contest_finalists (
    round_start date NOT NULL,
    entry_id uuid NOT NULL REFERENCES menu_contest_entries (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (round_start, entry_id)
);

-- 1人1票。投票期間中は入れ直せる。
CREATE TABLE menu_contest_votes (
    round_start date NOT NULL,
    voter_id uuid NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    entry_id uuid NOT NULL REFERENCES menu_contest_entries (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (round_start, voter_id)
);

CREATE INDEX menu_contest_votes_entry_idx ON menu_contest_votes (entry_id);

ALTER TABLE menu_contest_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE menu_contest_finalists ENABLE ROW LEVEL SECURITY;
ALTER TABLE menu_contest_votes ENABLE ROW LEVEL SECURITY;
