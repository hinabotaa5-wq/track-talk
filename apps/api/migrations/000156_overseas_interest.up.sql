-- 海外コーチの指導希望の集計（本体 #542）。1ユーザー1件（profile_id が主キー）。
-- 登録数は利用者に見せない。開始時のお知らせとパイロットの個別案内に使う。
CREATE TABLE overseas_interests (
    profile_id uuid PRIMARY KEY REFERENCES profiles(id) ON DELETE CASCADE,
    kind text NOT NULL DEFAULT 'both',
    CONSTRAINT overseas_interests_kind_check CHECK (kind IN ('monthly', 'single', 'both')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE overseas_interests ENABLE ROW LEVEL SECURITY;
