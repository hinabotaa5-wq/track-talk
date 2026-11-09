-- 大会までプランの無料相談。選手が構造化フォームで相談し、コーチが提案か辞退を返す。
-- 契約は選手が提案を受けたときに作る（既存のサポートプランの契約に接続する）。

-- コーチの相談の受付設定。受け入れ状況は対応可能カレンダーだけで決める（人数の上限は置かない）。
CREATE TABLE support_consult_settings (
    coach_id uuid PRIMARY KEY REFERENCES coach_profiles (profile_id) ON DELETE CASCADE,
    accepting boolean NOT NULL DEFAULT false,
    events text[] NOT NULL DEFAULT '{}',
    description text NOT NULL DEFAULT '' CHECK (char_length(description) <= 1000),
    -- 価格の目安。実際の価格は相談後の提案でコーチが決める。
    price_min_yen integer,
    price_max_yen integer,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((price_min_yen IS NULL) = (price_max_yen IS NULL)),
    CHECK (price_min_yen IS NULL OR (price_min_yen BETWEEN 2000 AND 9999999 AND price_max_yen BETWEEN price_min_yen AND 9999999))
);

-- 無料相談。requested: コーチの提案か辞退を待っている、clarifying: コーチの聞き直しに選手が答えるのを待っている、
-- proposed: 提案を出した、accepted: 選手が提案を受けて契約の申込みになった、
-- declined: コーチが辞退した、expired: 期限が過ぎた、withdrawn: 選手が取り下げた。
CREATE TABLE support_consults (
    id uuid PRIMARY KEY,
    athlete_id uuid NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    coach_id uuid NOT NULL REFERENCES coach_profiles (profile_id) ON DELETE CASCADE,
    status text NOT NULL DEFAULT 'requested',
    minor boolean NOT NULL,
    event text NOT NULL,
    answers jsonb NOT NULL,
    -- コーチが提案か辞退を返す期限（48時間）。聞き直しの答えが届いたら数え直す。聞き直し中は NULL。
    answer_by timestamptz,
    contract_id uuid REFERENCES coaching_contracts (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (status IN ('requested', 'clarifying', 'proposed', 'accepted', 'declined', 'expired', 'withdrawn')),
    CHECK (athlete_id <> coach_id),
    CHECK (status <> 'requested' OR answer_by IS NOT NULL)
);
-- 進行中の相談は、同じコーチに1つまで。
CREATE UNIQUE INDEX support_consults_open_idx ON support_consults (athlete_id, coach_id) WHERE status IN ('requested', 'clarifying', 'proposed');
CREATE INDEX support_consults_coach_idx ON support_consults (coach_id, status, created_at DESC);
CREATE INDEX support_consults_athlete_idx ON support_consults (athlete_id, created_at DESC);

-- コーチの聞き直し（フォーム形式の追加質問）。
CREATE TABLE support_consult_questions (
    id uuid PRIMARY KEY,
    consult_id uuid NOT NULL REFERENCES support_consults (id) ON DELETE CASCADE,
    question text NOT NULL CHECK (char_length(btrim(question)) BETWEEN 1 AND 300),
    answer text CHECK (answer IS NULL OR char_length(btrim(answer)) BETWEEN 1 AND 500),
    asked_at timestamptz NOT NULL DEFAULT now(),
    answered_at timestamptz,
    CHECK ((answer IS NULL) = (answered_at IS NULL))
);
CREATE INDEX support_consult_questions_idx ON support_consult_questions (consult_id, asked_at);

-- コーチの提案。1つの相談に1つ。
CREATE TABLE support_consult_proposals (
    consult_id uuid PRIMARY KEY REFERENCES support_consults (id) ON DELETE CASCADE,
    competition_name text NOT NULL CHECK (char_length(btrim(competition_name)) BETWEEN 1 AND 100),
    support_start_date date NOT NULL,
    competition_date date NOT NULL,
    video_reviews smallint NOT NULL CHECK (video_reviews BETWEEN 1 AND 30),
    menu_updates text NOT NULL CHECK (menu_updates IN ('none', 'once', 'weekly')),
    reply_formats text[] NOT NULL DEFAULT '{}' CHECK (reply_formats <@ ARRAY['text', 'video']),
    call_count smallint NOT NULL DEFAULT 0 CHECK (call_count BETWEEN 0 AND 12),
    call_minutes smallint NOT NULL DEFAULT 30 CHECK (call_minutes IN (15, 30, 45, 60)),
    price_yen integer NOT NULL CHECK (price_yen BETWEEN 2000 AND 9999999),
    content_note text NOT NULL DEFAULT '' CHECK (char_length(content_note) <= 500),
    -- 選手の「コーチに聞きたいこと」への回答。
    answer_to_question text NOT NULL DEFAULT '' CHECK (char_length(answer_to_question) <= 1000),
    -- 提案の有効期限。選手側が申込みを済ませるまでの期限（日本時間のその日の終わりまで）。
    valid_until timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (competition_date >= support_start_date),
    CHECK (competition_date - support_start_date + 1 BETWEEN 1 AND 90)
);

-- 提案から契約を作るときの入れ物になるプラン。コーチごとに1つ。一覧には出さず、プラン管理の数にも入れない。
-- 契約の中身（期間・価格・内容）は提案の写しを契約に持たせるので、このプランの値は使わない。
ALTER TABLE coaching_plans ADD COLUMN consult_template boolean NOT NULL DEFAULT false;
CREATE UNIQUE INDEX coaching_plans_consult_template_idx ON coaching_plans (coach_id) WHERE consult_template;

ALTER TABLE support_consult_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE support_consults ENABLE ROW LEVEL SECURITY;
ALTER TABLE support_consult_questions ENABLE ROW LEVEL SECURITY;
ALTER TABLE support_consult_proposals ENABLE ROW LEVEL SECURITY;
