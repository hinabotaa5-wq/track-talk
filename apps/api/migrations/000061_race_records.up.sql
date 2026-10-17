-- 競技会の記録。本人だけが読み書きし、種目ごとの推移のグラフに使う。
-- 記録は整数で持つ: トラックは1/100秒、跳躍・投てきはcm、混成は点。
-- 風は0.1m/s単位(+1.8m/s は 18)。風の公認が関係する種目だけに入る。
CREATE TABLE race_records (
    id uuid PRIMARY KEY,
    author_id uuid NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    raced_on date NOT NULL,
    event text NOT NULL,
    mark integer NOT NULL,
    wind smallint,
    memo text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (raced_on >= DATE '1950-01-01'),
    CHECK (
        event IN (
            '100m', '200m', '400m', '800m', '1500m', '5000m', '10000m',
            '100mh', '110mh', '400mh', '3000msc',
            'long-jump', 'triple-jump', 'high-jump', 'pole-vault',
            'shot-put', 'discus', 'hammer', 'javelin',
            'heptathlon', 'decathlon', '4x100', '4x400'
        )
    ),
    CHECK (mark > 0),
    CHECK (wind IS NULL OR wind BETWEEN -99 AND 99),
    CHECK (wind IS NULL OR event IN ('100m', '200m', '100mh', '110mh', 'long-jump', 'triple-jump')),
    CHECK (char_length(memo) <= 300)
);

CREATE INDEX race_records_author_event_idx ON race_records (author_id, event, raced_on, created_at);

ALTER TABLE race_records ENABLE ROW LEVEL SECURITY;
