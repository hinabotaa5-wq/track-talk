CREATE TABLE profiles (
    id uuid PRIMARY KEY,
    nickname varchar(30) NOT NULL UNIQUE,
    bio text NOT NULL DEFAULT '',
    is_admin boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE post_categories (
    id text PRIMARY KEY,
    name text NOT NULL,
    sort_order int NOT NULL
);

CREATE TABLE competitions (
    id uuid PRIMARY KEY,
    name text NOT NULL,
    starts_on date NOT NULL,
    ends_on date,
    location text NOT NULL DEFAULT '',
    scope text NOT NULL CHECK (scope IN ('domestic', 'international')),
    summary text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX competitions_starts_on_idx ON competitions (starts_on DESC);

CREATE TABLE posts (
    id uuid PRIMARY KEY,
    author_id uuid NOT NULL REFERENCES profiles (id) ON DELETE RESTRICT,
    category_id text NOT NULL REFERENCES post_categories (id),
    competition_id uuid REFERENCES competitions (id) ON DELETE SET NULL,
    title varchar(120) NOT NULL,
    body text NOT NULL,
    like_count int NOT NULL DEFAULT 0,
    comment_count int NOT NULL DEFAULT 0,
    hidden_at timestamptz,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX posts_created_visible_idx ON posts (created_at DESC)
    WHERE deleted_at IS NULL AND hidden_at IS NULL;
CREATE INDEX posts_popular_visible_idx ON posts (like_count DESC, created_at DESC)
    WHERE deleted_at IS NULL AND hidden_at IS NULL;
CREATE INDEX posts_competition_idx ON posts (competition_id, created_at DESC)
    WHERE deleted_at IS NULL AND hidden_at IS NULL;
CREATE INDEX posts_author_idx ON posts (author_id, created_at DESC);
CREATE INDEX posts_category_idx ON posts (category_id, created_at DESC)
    WHERE deleted_at IS NULL AND hidden_at IS NULL;

CREATE TABLE comments (
    id uuid PRIMARY KEY,
    post_id uuid NOT NULL REFERENCES posts (id) ON DELETE CASCADE,
    author_id uuid NOT NULL REFERENCES profiles (id) ON DELETE RESTRICT,
    parent_id uuid REFERENCES comments (id),
    body text NOT NULL,
    hidden_at timestamptz,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX comments_post_created_idx ON comments (post_id, created_at);

CREATE TABLE post_likes (
    post_id uuid NOT NULL REFERENCES posts (id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (post_id, user_id)
);

CREATE TABLE reports (
    id uuid PRIMARY KEY,
    reporter_id uuid NOT NULL REFERENCES profiles (id) ON DELETE RESTRICT,
    target_type text NOT NULL CHECK (target_type IN ('post', 'comment')),
    target_id uuid NOT NULL,
    reason text NOT NULL CHECK (reason IN ('spam', 'harassment', 'misinfo', 'other')),
    detail text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'reviewed', 'actioned', 'dismissed')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (reporter_id, target_type, target_id)
);

INSERT INTO post_categories (id, name, sort_order) VALUES
    ('race_impression', 'レース・大会の感想', 1),
    ('record_discussion', '記録・自己ベストについての議論', 2),
    ('athlete_discussion', '選手についての議論', 3),
    ('general', '陸上競技全般', 4);

INSERT INTO competitions (id, name, starts_on, ends_on, location, scope, summary) VALUES
    ('11111111-1111-4111-8111-111111111111', '世界陸上競技選手権大会', '2025-09-13', '2025-09-21', '東京', 'international', '世界陸連が主催する世界選手権。種目横断でトップ選手が集まる。'),
    ('22222222-2222-4222-8222-222222222222', 'ダイヤモンドリーグ', '2026-05-01', '2026-09-30', '各地', 'international', '世界陸連公認のトップミーティングシリーズ。'),
    ('33333333-3333-4333-8333-333333333333', '日本陸上競技選手権大会', '2026-06-26', '2026-06-28', '国立競技場', 'domestic', '日本選手権。国内の第一人者が集う主要大会。'),
    ('44444444-4444-4444-8444-444444444444', '日本選手権リレー', '2026-10-01', '2026-10-02', '横浜', 'domestic', 'リレー種目に特化した国内主要大会。');

ALTER TABLE profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE post_categories ENABLE ROW LEVEL SECURITY;
ALTER TABLE competitions ENABLE ROW LEVEL SECURITY;
ALTER TABLE posts ENABLE ROW LEVEL SECURITY;
ALTER TABLE comments ENABLE ROW LEVEL SECURITY;
ALTER TABLE post_likes ENABLE ROW LEVEL SECURITY;
ALTER TABLE reports ENABLE ROW LEVEL SECURITY;
