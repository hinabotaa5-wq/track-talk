-- コーチの「ひとこと」専用欄（本体 #649）。承認後の自己編集と公開表示で使う。空欄可。
-- 既存コーチは空で始め、sns_urls からの自動移行はしない。
ALTER TABLE coach_profiles
    ADD COLUMN hitokoto text NOT NULL DEFAULT '';

ALTER TABLE coach_profiles
    ADD CONSTRAINT coach_profiles_hitokoto_length CHECK (char_length(hitokoto) <= 100);
