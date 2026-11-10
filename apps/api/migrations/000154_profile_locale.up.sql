-- 表示言語。利用者の設定として保持する（当面 ja / en）。既存の利用者は ja のまま。
ALTER TABLE profiles
    ADD COLUMN locale text NOT NULL DEFAULT 'ja',
    ADD CONSTRAINT profiles_locale_check CHECK (locale IN ('ja', 'en'));
