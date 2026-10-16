-- 感覚メモ。練習で体を動かして感じた感覚を、本人だけが書き溜める。投稿（posts）とは別で、誰にも公開しない。
-- 書いた日（日本時間）は created_at から求める。種目は任意で、練習相手募集と同じコードを使う。
CREATE TABLE sense_notes (
    id uuid PRIMARY KEY,
    author_id uuid NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    body text NOT NULL,
    event text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (char_length(btrim(body)) BETWEEN 1 AND 300),
    CHECK (
        event IS NULL
        OR event IN (
            '100m',
            '200m',
            '400m',
            '800m',
            '1500m',
            '5000m',
            '10000m',
            '100mh',
            '110mh',
            '400mh',
            '3000msc',
            'long-jump',
            'triple-jump',
            'high-jump',
            'pole-vault',
            'shot-put',
            'discus',
            'hammer',
            'javelin',
            'heptathlon',
            'decathlon',
            '4x100',
            '4x400',
            'other'
        )
    )
);

CREATE INDEX sense_notes_author_created_idx ON sense_notes (author_id, created_at DESC, id DESC);

ALTER TABLE sense_notes ENABLE ROW LEVEL SECURITY;
