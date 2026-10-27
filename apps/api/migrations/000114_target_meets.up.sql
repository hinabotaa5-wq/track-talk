-- メモタブの「〇〇大会まであと何日」。1人1大会で、別の大会をセットすると置き換わる。
-- 都道府県の大会は取り込み直しで id が変わることがあるので、セットした時点の名前と日付を写して持つ。
CREATE TABLE target_meets (
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
