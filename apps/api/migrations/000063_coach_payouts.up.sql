-- コーチへの報酬と、月ごとの送金（Stripe Connect）。
-- 支払いは TrackTalk が売り手として受け取り（separate charges and transfers）、返却や期間の終わりごとに報酬を記録して、
-- 月に1回、前の月までの分をまとめてコーチの Stripe アカウントへ送金する。

-- stripe_account_id: コーチの受け取り用のアカウント（Express）。payouts_enabled: 本人確認と口座の登録が済み、送金を受け取れる。
-- reduced_fee_until: 最初の数人のコーチは初年の手数料を10%にする（2026-10-17 決定）。この日時までに始まったパーソナル指導が10%。
ALTER TABLE coach_profiles
    ADD COLUMN stripe_account_id text UNIQUE,
    ADD COLUMN payouts_enabled boolean NOT NULL DEFAULT false,
    ADD COLUMN reduced_fee_until timestamptz;

-- 月ごとの送金。税務のため、退会しても残す（コーチとの紐づけだけ外れる）。
CREATE TABLE coach_payouts (
    id uuid PRIMARY KEY,
    coach_id uuid REFERENCES profiles (id) ON DELETE SET NULL,
    -- 送金した分の月（日本時間の月初の日付）。この月までの報酬をまとめる。
    month date NOT NULL,
    amount_yen integer NOT NULL CHECK (amount_yen > 0),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'paid', 'failed')),
    stripe_transfer_id text UNIQUE,
    failure text,
    created_at timestamptz NOT NULL DEFAULT now(),
    paid_at timestamptz,
    UNIQUE (coach_id, month),
    CHECK (status <> 'paid' OR (stripe_transfer_id IS NOT NULL AND paid_at IS NOT NULL))
);

-- 1件ごとの報酬。添削は返却して代金が確定した時、パーソナル指導は期間が終わった時（途中で返金したら残りの分）に記録する。
CREATE TABLE coach_earnings (
    id uuid PRIMARY KEY,
    coach_id uuid REFERENCES profiles (id) ON DELETE SET NULL,
    source text NOT NULL CHECK (source IN ('review', 'personal')),
    review_request_id uuid UNIQUE,
    contract_id uuid UNIQUE,
    -- 利用者が払った額（返金を引いたもの）と、コーチの取り分の割合（%）。
    gross_yen integer NOT NULL CHECK (gross_yen > 0),
    share_percent smallint NOT NULL CHECK (share_percent BETWEEN 1 AND 100),
    amount_yen integer NOT NULL CHECK (amount_yen > 0),
    payout_id uuid REFERENCES coach_payouts (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    -- 添削の報酬に契約、パーソナル指導の報酬に添削の依頼は付かない（依頼や契約が消えても記録は残すので、NULL は許す）
    CHECK (source = 'review' OR review_request_id IS NULL),
    CHECK (source = 'personal' OR contract_id IS NULL)
);

CREATE INDEX coach_earnings_coach_idx ON coach_earnings (coach_id, created_at);
CREATE INDEX coach_earnings_unpaid_idx ON coach_earnings (created_at) WHERE payout_id IS NULL;

ALTER TABLE coach_payouts ENABLE ROW LEVEL SECURITY;
ALTER TABLE coach_earnings ENABLE ROW LEVEL SECURITY;
