-- パーソナル指導のプラン。コーチが自分で作り、値段も決める。
ALTER TABLE coach_profiles
    -- パーソナル指導の誓約（外部の連絡先を交換しない、会う約束をしない、連絡先の自動検出に同意する）をした時刻と文面の版。
    -- 誓約していないコーチはプランを作れない。
    ADD COLUMN pledged_at timestamptz,
    ADD COLUMN pledge_version smallint,
    ADD CONSTRAINT coach_profiles_pledge_check CHECK ((pledged_at IS NULL) = (pledge_version IS NULL));

CREATE TABLE coaching_plans (
    id uuid PRIMARY KEY,
    coach_id uuid NOT NULL REFERENCES coach_profiles (profile_id) ON DELETE CASCADE,
    -- period: 2週間か1か月の期間プラン、single: 7日以内に終わる単発。
    kind text NOT NULL,
    title text NOT NULL,
    description text NOT NULL,
    duration_days smallint NOT NULL,
    -- 上限は決めない（ひなたさん決定）。桁の打ち間違いだけ防ぐため、決済で扱える額までにする。
    price_yen integer NOT NULL,
    -- 同時に受ける人数の上限。
    capacity smallint NOT NULL,
    published boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (kind IN ('period', 'single')),
    CHECK ((kind = 'period' AND duration_days IN (14, 30)) OR (kind = 'single' AND duration_days = 7)),
    CHECK (char_length(btrim(title)) BETWEEN 1 AND 60),
    CHECK (char_length(btrim(description)) BETWEEN 1 AND 1000),
    CHECK (price_yen BETWEEN 2000 AND 9999999),
    CHECK (capacity BETWEEN 1 AND 50)
);

CREATE INDEX coaching_plans_coach_idx ON coaching_plans (coach_id, created_at);

ALTER TABLE coaching_plans ENABLE ROW LEVEL SECURITY;
