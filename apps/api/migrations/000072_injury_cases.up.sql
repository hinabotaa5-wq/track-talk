-- ケガ部の「データ」タブ。ケガをしたときの状況と、そのときどうすればよかったかを、選択式だけで集める。
-- 自由記述・日付・学校・場所は聞かない(未成年が多く、個人が特定されないようにするため)。
-- user_id は上限の確認と本人の削除のためだけに持ち、API では返さない。退会しても回答は残し、紐づけだけを外す。
CREATE TABLE injury_cases (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid REFERENCES profiles (id) ON DELETE SET NULL,
    body_part text NOT NULL CHECK (
        body_part IN ('hamstring-strain', 'quad-strain', 'sprain', 'achilles-tendinitis', 'jumpers-knee', 'shin-splints', 'shoulder', 'elbow', 'other')
    ),
    timing text NOT NULL CHECK (timing IN ('practice', 'warmup', 'race', 'daily', 'other')),
    situations text[] NOT NULL CHECK (cardinality(situations) BETWEEN 1 AND 4),
    preventions text[] NOT NULL CHECK (cardinality(preventions) BETWEEN 1 AND 4),
    hidden_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX injury_cases_user_idx ON injury_cases (user_id, created_at DESC);
CREATE INDEX injury_cases_part_idx ON injury_cases (body_part) WHERE hidden_at IS NULL;
