-- オンラインコーチング（おまかせ添削）の土台。
-- 生まれた年月は月の1日で持つ。18歳未満かどうかの判定と、保護者の同意が要るかに使う。
ALTER TABLE profiles
    ADD COLUMN birth_month date,
    ADD CONSTRAINT profiles_birth_month_check CHECK (
        birth_month IS NULL
        OR (birth_month = date_trunc('month', birth_month)::date AND birth_month >= DATE '1900-01-01')
    );

-- 18歳未満の利用者の保護者の同意。1人につき1行で、リンクを出し直すとトークンだけ入れ替わる。
-- リンクは本人が保護者に送る。トークンそのものは保存せず、SHA-256 だけを持つ。
CREATE TABLE guardian_consents (
    minor_id uuid PRIMARY KEY REFERENCES profiles (id) ON DELETE CASCADE,
    token_hash text NOT NULL UNIQUE,
    status text NOT NULL DEFAULT 'pending',
    guardian_name text,
    relationship text,
    -- NULL は上限なし。
    monthly_limit_yen integer,
    allow_publish boolean NOT NULL DEFAULT false,
    consented_at timestamptz,
    declined_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (status IN ('pending', 'consented', 'declined', 'revoked')),
    CHECK (relationship IS NULL OR relationship IN ('father', 'mother', 'other')),
    CHECK (guardian_name IS NULL OR char_length(btrim(guardian_name)) BETWEEN 1 AND 50),
    CHECK (monthly_limit_yen IS NULL OR monthly_limit_yen IN (1000, 3000, 5000)),
    CHECK (status <> 'consented' OR (guardian_name IS NOT NULL AND relationship IS NOT NULL AND consented_at IS NOT NULL))
);

ALTER TABLE guardian_consents ENABLE ROW LEVEL SECURITY;

-- Web プッシュの宛先。端末（ブラウザ）ごとに1行。
CREATE TABLE push_subscriptions (
    id uuid PRIMARY KEY,
    profile_id uuid NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    endpoint text NOT NULL UNIQUE,
    p256dh text NOT NULL,
    auth text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (endpoint LIKE 'https://%' AND char_length(endpoint) <= 1000),
    CHECK (char_length(p256dh) BETWEEN 1 AND 200),
    CHECK (char_length(auth) BETWEEN 1 AND 100)
);

CREATE INDEX push_subscriptions_profile_idx ON push_subscriptions (profile_id);

ALTER TABLE push_subscriptions ENABLE ROW LEVEL SECURITY;
