-- 英語表示で出すコーチプロフィールの自動翻訳（本体 #824）。原文は coach_profiles / profiles に残し、訳だけを別テーブルに置く。
CREATE TABLE coach_profile_translations (
    coach_id uuid PRIMARY KEY REFERENCES coach_profiles(profile_id) ON DELETE CASCADE,
    source_hash text NOT NULL,
    nickname_en text NOT NULL DEFAULT '',
    hitokoto_en text NOT NULL DEFAULT '',
    achievements_en text NOT NULL DEFAULT '',
    best_record_en text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE coach_profile_translations ENABLE ROW LEVEL SECURITY;

-- 外部AIの呼び出し回数だけを記録。本文・利用者ID・応答は保存しない（sense_summary_attempts と同方式）。
CREATE TABLE coach_profile_translation_attempts (
    id bigserial PRIMARY KEY,
    attempted_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX coach_profile_translation_attempts_attempted_at_idx ON coach_profile_translation_attempts (attempted_at);
ALTER TABLE coach_profile_translation_attempts ENABLE ROW LEVEL SECURITY;
