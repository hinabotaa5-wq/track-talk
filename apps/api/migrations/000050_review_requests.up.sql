-- おまかせ添削の依頼（決済なしのデモ版）。決済ができるまでは、運営が配った招待コードを入れた人だけが依頼できる。

-- コーチの受付の状態。on_duty の人に、最後に打診した時刻が古い順で依頼を回す。
ALTER TABLE coach_profiles
    ADD COLUMN on_duty boolean NOT NULL DEFAULT false,
    ADD COLUMN daily_cap smallint NOT NULL DEFAULT 5,
    ADD COLUMN last_offered_at timestamptz,
    ADD CONSTRAINT coach_profiles_daily_cap_check CHECK (daily_cap BETWEEN 1 AND 20),
    ADD CONSTRAINT coach_profiles_on_duty_check CHECK (NOT on_duty OR status = 'approved');

-- デモの参加者。招待コードを使うと付く。
ALTER TABLE profiles
    ADD COLUMN coaching_demo boolean NOT NULL DEFAULT false;

-- デモの招待コード。1つのコードは1人だけが使える。
CREATE TABLE coaching_demo_codes (
    code text PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT now(),
    used_by uuid REFERENCES profiles (id) ON DELETE SET NULL,
    used_at timestamptz,
    CHECK (code ~ '^[A-Z0-9]{8}$'),
    -- 使った人が退会しても、コードは使用済みのまま残す。
    CHECK (used_by IS NULL OR used_at IS NOT NULL)
);

ALTER TABLE coaching_demo_codes ENABLE ROW LEVEL SECURITY;

-- 依頼の本体。状態の流れは online-coaching/design-review.md の「3. 依頼の状態」のとおり。
CREATE TABLE review_requests (
    id uuid PRIMARY KEY,
    requester_id uuid NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    tier text NOT NULL,
    event text NOT NULL,
    focus text[] NOT NULL,
    video_public_id text NOT NULL UNIQUE,
    status text NOT NULL DEFAULT 'queued',
    due_at timestamptz NOT NULL,
    coach_id uuid REFERENCES profiles (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (tier IN ('coach', 'senpai')),
    CHECK (event IN (
        '100m', '200m', '400m', '800m', '1500m', '5000m', '10000m',
        '110mh', '100mh', '400mh', '3000msc',
        'high-jump', 'pole-vault', 'long-jump', 'triple-jump',
        'shot-put', 'discus', 'hammer', 'javelin', 'race-walk', 'relay'
    )),
    CHECK (
        cardinality(focus) BETWEEN 1 AND 8
        AND focus <@ ARRAY['start', 'acceleration', 'top-speed', 'takeoff', 'arm', 'contact', 'rhythm', 'other']::text[]
    ),
    CHECK (status IN ('queued', 'offered', 'in_review', 'returned', 'clarifying', 'expired'))
);

CREATE INDEX review_requests_requester_idx ON review_requests (requester_id, created_at DESC);
CREATE INDEX review_requests_queue_idx ON review_requests (status, created_at) WHERE status = 'queued';

ALTER TABLE review_requests ENABLE ROW LEVEL SECURITY;
