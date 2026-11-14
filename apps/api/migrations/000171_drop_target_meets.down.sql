-- #704 の down: 000114 のスキーマに戻す。up で消えた行は復元しない。
CREATE TABLE IF NOT EXISTS target_meets (
    profile_id uuid PRIMARY KEY REFERENCES profiles (id) ON DELETE CASCADE,
    source text NOT NULL CHECK (source IN ('major', 'competition')),
    -- 国内主要大会は "slug:year"、都道府県の大会は competitions.id。
    ref text NOT NULL,
    name text NOT NULL,
    starts_on date NOT NULL,
    ends_on date,
    updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE target_meets ENABLE ROW LEVEL SECURITY;
