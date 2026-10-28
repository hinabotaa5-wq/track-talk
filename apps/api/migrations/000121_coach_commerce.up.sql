-- コーチごとの特定商取引法の表記(サポートプランはコーチが販売者)。
-- 表記の要否はコーチの自己申告で、既定は「表記要」。運営は事業の実態が見えたら表記要に戻せる。
CREATE TABLE coach_commerce_settings (
    coach_id uuid PRIMARY KEY REFERENCES coach_profiles (profile_id) ON DELETE CASCADE,
    disclosure_required boolean NOT NULL DEFAULT true,
    -- 表記不要を申告したときのチェックリストの版と日時。
    exempt_checklist_version smallint,
    exempt_declared_at timestamptz,
    -- 運営が表記要に戻した日時。戻したあとは本人は不要に申告し直せない。
    forced_required_at timestamptz,
    forced_by uuid REFERENCES profiles (id) ON DELETE SET NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (disclosure_required OR (exempt_checklist_version IS NOT NULL AND exempt_declared_at IS NOT NULL AND forced_required_at IS NULL))
);

-- 本名・住所・電話番号。表示用のデータと分け、アプリで暗号化してから入れる。
-- 公開の API はこのテーブルを読まない(表記要のコーチの表記だけ、専用の関数を通して出す)。
-- 全コーチがプランの公開前に登録する。表記不要のコーチの分は、開示請求と紛争対応にだけ使う。
CREATE TABLE coach_legal_identities (
    coach_id uuid PRIMARY KEY REFERENCES coach_profiles (profile_id) ON DELETE CASCADE,
    key_id text NOT NULL,
    legal_name_enc bytea NOT NULL,
    address_enc bytea NOT NULL,
    phone_enc bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- 運営が登録情報を見たときの記録。
CREATE TABLE coach_identity_access_logs (
    id uuid PRIMARY KEY,
    coach_id uuid NOT NULL,
    admin_id uuid REFERENCES profiles (id) ON DELETE SET NULL,
    purpose text NOT NULL CHECK (char_length(purpose) BETWEEN 1 AND 200),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX coach_identity_access_logs_coach_idx ON coach_identity_access_logs (coach_id, created_at DESC);

ALTER TABLE coach_commerce_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE coach_legal_identities ENABLE ROW LEVEL SECURITY;
ALTER TABLE coach_identity_access_logs ENABLE ROW LEVEL SECURITY;
