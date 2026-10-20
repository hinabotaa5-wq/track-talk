-- 感覚メモの1週間のまとめ。本人が頼んだときに、直近7日（日本時間）の感覚メモを AI（Gemini）でまとめて残す。
-- 本人だけが読む。作り直すたびに行を足し、1日に作れる回数はこの表の行で数える。
CREATE TABLE sense_note_summaries (
    id uuid PRIMARY KEY,
    author_id uuid NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    from_day date NOT NULL,
    to_day date NOT NULL,
    note_count integer NOT NULL CHECK (note_count > 0),
    -- {"overview":"","good":[],"concerns":[],"next":[]}
    summary jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (from_day <= to_day)
);

CREATE INDEX sense_note_summaries_author_created_idx ON sense_note_summaries (author_id, created_at DESC);

ALTER TABLE sense_note_summaries ENABLE ROW LEVEL SECURITY;
