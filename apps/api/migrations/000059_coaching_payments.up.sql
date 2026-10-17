-- パーソナル指導の支払い（Stripe Checkout）。
-- Stripe の鍵があるときにコーチが承諾したものは payment_required を立て、支払いが済んだら期間が始まる。
-- refunded: 返金して途中で閉じた（コーチが72時間書かなかった、運営が返金した）。
ALTER TABLE coaching_contracts
    ADD COLUMN payment_required boolean NOT NULL DEFAULT false,
    DROP CONSTRAINT coaching_contracts_status_check,
    ADD CONSTRAINT coaching_contracts_status_check CHECK (status IN (
        'applied', 'accepted', 'active', 'suspended', 'ended', 'refunded', 'declined', 'guardian_declined', 'expired', 'cancelled'
    )),
    DROP CONSTRAINT coaching_contracts_period_check,
    ADD CONSTRAINT coaching_contracts_period_check CHECK (
        status NOT IN ('active', 'suspended', 'ended', 'refunded') OR (starts_at IS NOT NULL AND ends_at IS NOT NULL)
    );

-- 支払いの記録。会計のために、退会などで契約が消えても残す。
CREATE TABLE coaching_payments (
    id uuid PRIMARY KEY,
    contract_id uuid REFERENCES coaching_contracts (id) ON DELETE SET NULL,
    -- 払った人。18歳未満の契約は保護者が払う。
    payer text NOT NULL CHECK (payer IN ('requester', 'guardian')),
    amount_yen integer NOT NULL CHECK (amount_yen > 0),
    checkout_session_id text NOT NULL UNIQUE,
    checkout_url text NOT NULL,
    session_expires_at timestamptz NOT NULL,
    payment_intent_id text UNIQUE,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'paid', 'refunded', 'partially_refunded')),
    refunded_yen integer NOT NULL DEFAULT 0 CHECK (refunded_yen >= 0 AND refunded_yen <= amount_yen),
    -- 返金の理由。no_coach_reply: コーチが72時間書かなかった。not_started: 支払いが期限後などで始められなかった。
    -- admin_full / admin_remaining: 運営が全額・残りの日数分を返金した。
    refund_reason text CHECK (refund_reason IN ('no_coach_reply', 'not_started', 'admin_full', 'admin_remaining')),
    paid_at timestamptz,
    refunded_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (status = 'pending' OR (payment_intent_id IS NOT NULL AND paid_at IS NOT NULL)),
    CHECK ((status IN ('refunded', 'partially_refunded')) = (refunded_yen > 0))
);

CREATE INDEX coaching_payments_contract_idx ON coaching_payments (contract_id, created_at DESC);

ALTER TABLE coaching_payments ENABLE ROW LEVEL SECURITY;
