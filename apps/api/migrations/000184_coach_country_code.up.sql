-- コーチの国（本体 #932）。一覧の名前の前に国旗を出すために使う。ISO 3166-1 alpha-2 の大文字2字。
-- コーチ本人は入力・変更しない。運営が管理画面から設定する。既存・新規とも日本（JP）で始める。
ALTER TABLE coach_profiles
    ADD COLUMN country_code text NOT NULL DEFAULT 'JP';

ALTER TABLE coach_profiles
    ADD CONSTRAINT coach_profiles_country_code_format CHECK (country_code ~ '^[A-Z]{2}$');
