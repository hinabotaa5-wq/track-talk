-- レビューを書いた人の種目・レベル・足のサイズ・足幅・使い方。どれも任意で、絞り込みに使う。
-- 商品の公式ページの仕様は取り込まない(商品の更新でページが変わるため)。絞り込みはレビュー側の情報だけで行う。
-- 足のサイズは mm で持つ(22.0〜30.0cm を 0.5cm 刻み)。
ALTER TABLE product_reviews
    ADD COLUMN event text,
    ADD COLUMN level text,
    ADD COLUMN foot_size smallint,
    ADD COLUMN foot_width text,
    ADD COLUMN usage text,
    ADD CONSTRAINT product_reviews_event_check CHECK (
        event IS NULL
        OR event IN (
            '100m', '200m', '400m', '800m', '1500m', '5000m', '10000m',
            '100mh', '110mh', '400mh', '3000msc',
            'long-jump', 'triple-jump', 'high-jump', 'pole-vault',
            'shot-put', 'discus', 'hammer', 'javelin',
            'heptathlon', 'decathlon', '4x100', '4x400', 'other'
        )
    ),
    ADD CONSTRAINT product_reviews_level_check CHECK (
        level IS NULL OR level IN ('junior-high', 'high-school', 'university', 'adult', 'masters')
    ),
    ADD CONSTRAINT product_reviews_foot_size_check CHECK (
        foot_size IS NULL OR (foot_size BETWEEN 220 AND 300 AND foot_size % 5 = 0)
    ),
    ADD CONSTRAINT product_reviews_foot_width_check CHECK (
        foot_width IS NULL OR foot_width IN ('narrow', 'regular', 'wide')
    ),
    ADD CONSTRAINT product_reviews_usage_check CHECK (
        usage IS NULL OR usage IN ('practice', 'race', 'both')
    );
