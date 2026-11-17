-- 海外コーチ（コーチの profiles.locale = 'en'）とのやり取りの自動翻訳（本体 #916）。
-- 原文のテーブルは変えず、訳だけを別テーブルに置く。各 source_hash は訳したときの原文の sha256 hex で、
-- 読み取り時に現在の原文と一致するときだけ訳を返す。
-- 添削の確認は実物に合わせ、コーチの差し戻しの質問（review_requests.info_request）を訳す（2026-11-18 運営者判断）。
-- 選手の補足は再提出で書き換わる相談文（question）なので、question_en の作り直しで扱う。
-- 追加質問の確認も実物に合わせ、clarify_body（選手の確認質問）は英訳、clarification（コーチの補足回答）は和訳する。
CREATE TABLE review_translations (
    request_id uuid PRIMARY KEY REFERENCES review_requests(id) ON DELETE CASCADE,
    question_en text NOT NULL DEFAULT '',
    question_source_hash text,
    good_point_ja text NOT NULL DEFAULT '',
    fix_points_ja text[],
    drill_ja text NOT NULL DEFAULT '',
    response_source_hash text,
    info_request_ja text NOT NULL DEFAULT '',
    info_request_source_hash text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE review_translations ENABLE ROW LEVEL SECURITY;

CREATE TABLE consultation_translations (
    consultation_id uuid PRIMARY KEY REFERENCES coaching_consultations(id) ON DELETE CASCADE,
    question_en text NOT NULL DEFAULT '',
    question_source_hash text,
    answer_ja text NOT NULL DEFAULT '',
    answer_source_hash text,
    clarify_body_en text NOT NULL DEFAULT '',
    clarify_source_hash text,
    clarification_ja text NOT NULL DEFAULT '',
    clarification_source_hash text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE consultation_translations ENABLE ROW LEVEL SECURITY;

CREATE TABLE coaching_message_translations (
    message_id uuid PRIMARY KEY REFERENCES coaching_messages(id) ON DELETE CASCADE,
    body_en text NOT NULL DEFAULT '',
    body_ja text NOT NULL DEFAULT '',
    menu_ja jsonb,
    source_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE coaching_message_translations ENABLE ROW LEVEL SECURITY;

-- 外部AIの呼び出し回数だけを記録。本文・利用者ID・応答は保存しない（coach_profile_translation_attempts と同形）。
CREATE TABLE coaching_translation_attempts (
    id bigserial PRIMARY KEY,
    attempted_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX coaching_translation_attempts_attempted_at_idx ON coaching_translation_attempts (attempted_at);
ALTER TABLE coaching_translation_attempts ENABLE ROW LEVEL SECURITY;
