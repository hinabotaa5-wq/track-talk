-- 18歳未満が自分で払う動画添削・追加相談の保護者同意（フル版）。
-- 保護者が同意用のリンクから同意した時の IP と規約の版、同意に使ったリンク（同じリンクでは二度同意できない）を残す。
-- self_pay_limit_yen は、本人が自分で払う動画添削・追加相談の月の上限。1,500／3,000／5,000／10,000 円から保護者が選ぶ。
-- monthly_limit_yen（保護者が払うパーソナル指導・サポートプランなどにかかる上限）は、これまでどおり別に残す。
ALTER TABLE guardian_consents
    ADD COLUMN self_pay_limit_yen integer NOT NULL DEFAULT 1500,
    ADD CONSTRAINT guardian_consents_self_pay_limit_yen_check CHECK (self_pay_limit_yen IN (1500, 3000, 5000, 10000)),
    ADD COLUMN consent_ip text,
    ADD COLUMN terms_version text,
    ADD COLUMN consent_token_hash text,
    ADD COLUMN limit_request_sent_at timestamptz;

-- 同意の証跡。リンクの送信・同意・同意しない・上限の変更・撤回・上限変更のお願いを、起きた順に1行ずつ残す（書き換えない）。
-- 上限は、本人が自分で払う分の上限（self_pay_limit_yen）を残す。
CREATE TABLE guardian_consent_events (
    id bigserial PRIMARY KEY,
    minor_id uuid NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    event text NOT NULL,
    guardian_email text,
    guardian_name text,
    relationship text,
    self_pay_limit_yen integer,
    previous_self_pay_limit_yen integer,
    terms_version text,
    ip text,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (event IN ('link_sent', 'consented', 'declined', 'limit_changed', 'revoked', 'limit_request_sent'))
);
CREATE INDEX guardian_consent_events_minor_idx ON guardian_consent_events (minor_id, created_at);
ALTER TABLE guardian_consent_events ENABLE ROW LEVEL SECURITY;

-- 保護者が上限の変更・同意の撤回をするためのリンク。同意の確認・購入の通知・上限変更のお願いのメールに入れる。
-- 同意用のリンクとは別物で、これでは同意そのものはできない。トークンはハッシュだけを持つ。
CREATE TABLE guardian_manage_links (
    token_hash text PRIMARY KEY,
    minor_id uuid NOT NULL REFERENCES guardian_consents (minor_id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX guardian_manage_links_minor_idx ON guardian_manage_links (minor_id);
ALTER TABLE guardian_manage_links ENABLE ROW LEVEL SECURITY;

-- 保護者に送ったメールの記録。購入の通知は支払いと同じトランザクションで pending の行を作り、送れたら sent にする。
-- 送れなかったものは tick が送り直す。同じ購入の通知は1回だけ（kind・ref_type・ref_id で一意）。
CREATE TABLE guardian_notices (
    id uuid PRIMARY KEY,
    minor_id uuid NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    kind text NOT NULL,
    ref_type text,
    ref_id uuid,
    guardian_email text NOT NULL,
    amount_yen integer,
    coach_name text,
    purchased_at timestamptz,
    status text NOT NULL DEFAULT 'pending',
    attempts integer NOT NULL DEFAULT 0,
    last_error text,
    sent_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (kind IN ('consent_request', 'consented', 'purchase', 'limit_request')),
    CHECK (status IN ('pending', 'sent', 'failed')),
    UNIQUE (kind, ref_type, ref_id)
);
CREATE INDEX guardian_notices_retry_idx ON guardian_notices (updated_at) WHERE status <> 'sent';
ALTER TABLE guardian_notices ENABLE ROW LEVEL SECURITY;
