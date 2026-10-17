-- パーソナル指導のやりとり欄と、終わったあとの評価。
-- 普段は運営も読まない（通報があったときだけ開ける。その仕組みは保護者と通報の PR で足す）。
CREATE TABLE coaching_messages (
    id uuid PRIMARY KEY,
    contract_id uuid NOT NULL REFERENCES coaching_contracts (id) ON DELETE CASCADE,
    -- 退会した人のメッセージは「退会したユーザー」として残し、相手の指導の記録が欠けないようにする。
    sender_id uuid REFERENCES profiles (id) ON DELETE SET NULL,
    sender_role text NOT NULL,
    body text NOT NULL DEFAULT '',
    -- 非公開の動画（添削と同じ coaching フォルダ）。
    video_public_id text UNIQUE,
    -- コーチが送る練習メニュー。{"title": "...", "days": [{"day": "...", "content": "..."}]}
    menu jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (sender_role IN ('requester', 'coach', 'guardian')),
    CHECK (char_length(body) <= 2000),
    CHECK (btrim(body) <> '' OR video_public_id IS NOT NULL OR menu IS NOT NULL),
    CHECK (menu IS NULL OR sender_role = 'coach')
);

CREATE INDEX coaching_messages_contract_idx ON coaching_messages (contract_id, created_at);

ALTER TABLE coaching_messages ENABLE ROW LEVEL SECURITY;

-- 連絡先の自動検出で送信を止めた回数。止めた文章そのものは持たない。
CREATE TABLE coaching_blocked_counts (
    contract_id uuid NOT NULL REFERENCES coaching_contracts (id) ON DELETE CASCADE,
    profile_id uuid NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    count integer NOT NULL DEFAULT 0,
    last_blocked_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (contract_id, profile_id)
);

ALTER TABLE coaching_blocked_counts ENABLE ROW LEVEL SECURITY;

-- 期間が終わったあと、利用者がつける★。
CREATE TABLE coaching_contract_ratings (
    contract_id uuid PRIMARY KEY REFERENCES coaching_contracts (id) ON DELETE CASCADE,
    stars smallint NOT NULL,
    rated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (stars BETWEEN 1 AND 5)
);

ALTER TABLE coaching_contract_ratings ENABLE ROW LEVEL SECURITY;
