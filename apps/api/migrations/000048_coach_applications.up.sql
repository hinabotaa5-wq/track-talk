-- コーチ（先輩を含む）の応募と審査。1人につき1行。
-- 応募で集めるのは担当種目・記録・成績・競技年数だけにする（本名や証明の画像は持たない）。
CREATE TABLE coach_profiles (
    profile_id uuid PRIMARY KEY REFERENCES profiles (id) ON DELETE CASCADE,
    -- coach: コーチ枠、senpai: 大学生・社会人の先輩枠。
    tier text NOT NULL,
    -- 担当種目。コードは練習相手募集の種目に合わせ、競歩(race-walk)とリレー(relay)を足したもの。
    events text[] NOT NULL,
    best_record text NOT NULL,
    achievements text NOT NULL,
    years smallint NOT NULL,
    status text NOT NULL DEFAULT 'pending',
    applied_at timestamptz NOT NULL DEFAULT now(),
    reviewed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (tier IN ('coach', 'senpai')),
    CHECK (
        cardinality(events) BETWEEN 1 AND 21
        AND events <@ ARRAY[
            '100m', '200m', '400m', '800m', '1500m', '5000m', '10000m', '110mh', '100mh', '400mh', '3000msc', 'high-jump', 'pole-vault', 'long-jump', 'triple-jump', 'shot-put', 'discus', 'hammer', 'javelin', 'race-walk', 'relay'
        ]::text[]
    ),
    CHECK (char_length(btrim(best_record)) BETWEEN 1 AND 200),
    CHECK (char_length(btrim(achievements)) BETWEEN 1 AND 500),
    CHECK (years BETWEEN 0 AND 60),
    CHECK (status IN ('pending', 'approved', 'rejected', 'suspended')),
    CHECK (status = 'pending' OR reviewed_at IS NOT NULL)
);

CREATE INDEX coach_profiles_status_idx ON coach_profiles (status, applied_at);

ALTER TABLE coach_profiles ENABLE ROW LEVEL SECURITY;
