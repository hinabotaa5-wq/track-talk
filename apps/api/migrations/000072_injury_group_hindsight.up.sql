-- ケガ共有に入るときの「いま思うと、どうすればよかったか」(選択式)。「データ」タブで集計して、まだケガをしていない人の予防に使う。
-- 自由記述は持たない。メンバーが抜けると一緒に消える。
CREATE TABLE injury_group_hindsight (
    user_id uuid NOT NULL,
    body_part text NOT NULL,
    codes text[] NOT NULL CHECK (
        cardinality(codes) BETWEEN 1 AND 8
        AND codes <@ ARRAY['rest-early', 'warmup', 'adjust-volume', 'see-doctor-early', 'sleep-food', 'gear', 'tell-coach', 'none']::text[]
    ),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, body_part),
    FOREIGN KEY (user_id, body_part) REFERENCES injury_group_members (user_id, body_part) ON DELETE CASCADE
);
