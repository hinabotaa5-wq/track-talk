-- パーソナル指導の申し込みから終わりまで。1回の申し込みが1行。
-- いまはデモ版なので、コーチが承諾するとそのまま期間が始まる（支払いは決済を入れる PR で足す）。
CREATE TABLE coaching_contracts (
    id uuid PRIMARY KEY,
    -- プランは消さず非公開にする運用なので、申し込みがあるプランは消せないようにする。
    plan_id uuid NOT NULL REFERENCES coaching_plans (id) ON DELETE RESTRICT,
    coach_id uuid NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    requester_id uuid NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    -- 申し込んだ時点のプランの写し。あとでコーチがプランを直しても、この申し込みの中身は変わらない。
    plan_kind text NOT NULL,
    plan_title text NOT NULL,
    duration_days smallint NOT NULL,
    price_yen integer NOT NULL,
    -- 申し込みの内容。
    event text NOT NULL,
    goal text NOT NULL,
    current_record text NOT NULL,
    message text NOT NULL,
    -- 申し込んだときに18歳未満だったか。コーチにも見せる。
    minor boolean NOT NULL,
    -- applied: コーチの返事待ち、active: 期間中、ended: 終わった、
    -- declined: コーチが断った、expired: 48時間返事がなかった、cancelled: 返事の前に利用者が取り下げた。
    status text NOT NULL DEFAULT 'applied',
    respond_by timestamptz NOT NULL,
    responded_at timestamptz,
    starts_at timestamptz,
    ends_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (plan_kind IN ('period', 'single')),
    CHECK (coach_id <> requester_id),
    CHECK (char_length(btrim(goal)) BETWEEN 1 AND 200),
    CHECK (char_length(btrim(current_record)) BETWEEN 1 AND 100),
    CHECK (char_length(btrim(message)) BETWEEN 1 AND 1000),
    CHECK (status IN ('applied', 'active', 'ended', 'declined', 'expired', 'cancelled')),
    CHECK (status NOT IN ('active', 'ended') OR (starts_at IS NOT NULL AND ends_at IS NOT NULL))
);

-- 同じコーチに、返事待ちか期間中の申し込みは1つまで。
CREATE UNIQUE INDEX coaching_contracts_open_idx ON coaching_contracts (requester_id, coach_id) WHERE status IN ('applied', 'active');
CREATE INDEX coaching_contracts_requester_idx ON coaching_contracts (requester_id, created_at DESC);
CREATE INDEX coaching_contracts_coach_idx ON coaching_contracts (coach_id, status, created_at DESC);
CREATE INDEX coaching_contracts_plan_idx ON coaching_contracts (plan_id) WHERE status IN ('applied', 'active');

ALTER TABLE coaching_contracts ENABLE ROW LEVEL SECURITY;

-- 契約を始めるときの同意。利用者は申し込みで、コーチは承諾で、それぞれ別に同意する。
-- report_review: 通報があったとき、運営がやりとりを読むこと。contact_detection: 連絡先を機械で検出して送信を止めること。
CREATE TABLE coaching_contract_consents (
    contract_id uuid NOT NULL REFERENCES coaching_contracts (id) ON DELETE CASCADE,
    party text NOT NULL,
    profile_id uuid NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    -- 同意した文面の版。
    version smallint NOT NULL,
    report_review boolean NOT NULL,
    contact_detection boolean NOT NULL,
    agreed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (contract_id, party),
    CHECK (party IN ('requester', 'coach')),
    CHECK (report_review AND contact_detection)
);

ALTER TABLE coaching_contract_consents ENABLE ROW LEVEL SECURITY;
