-- 個人情報保護法28条の同意(海外の委託先への提供など)を、いつ・どの版の説明で受けたか残す。
CREATE TABLE privacy_consents (
    user_id uuid NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    kind text NOT NULL,
    version text NOT NULL,
    consented_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, kind)
);
ALTER TABLE privacy_consents ENABLE ROW LEVEL SECURITY;
