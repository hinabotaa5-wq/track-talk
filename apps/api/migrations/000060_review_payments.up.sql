-- フォーム添削の支払い（Stripe Checkout）。Stripe の鍵があるときだけ使う。
-- awaiting_payment: 依頼は作ったが支払いがまだ。支払いが届いたら queued になって割り当てが始まる。
-- 払わないまま支払い画面が閉じたものは、tick が依頼ごと消す（動画を送り直せるように）。
ALTER TABLE review_requests
    DROP CONSTRAINT review_requests_status_check,
    ADD CONSTRAINT review_requests_status_check CHECK (status IN (
        'awaiting_payment', 'queued', 'offered', 'in_review', 'returned', 'clarifying', 'expired'
    ));

-- 1本ごとの支払い。カード（Apple Pay・Google Pay を含む）は送った時に仮押さえし、返却で確定、
-- 目安を過ぎたら取り消す。PayPay は仮押さえができないので、その場で払って、目安を過ぎたら返金する。
-- 会計のために、依頼や退会で消えても残す。
CREATE TABLE review_payments (
    id uuid PRIMARY KEY,
    request_id uuid REFERENCES review_requests (id) ON DELETE SET NULL,
    payer_id uuid REFERENCES profiles (id) ON DELETE SET NULL,
    method text NOT NULL CHECK (method IN ('card', 'paypay')),
    amount_yen integer NOT NULL CHECK (amount_yen > 0),
    checkout_session_id text NOT NULL UNIQUE,
    checkout_url text NOT NULL,
    session_expires_at timestamptz NOT NULL,
    payment_intent_id text UNIQUE,
    -- pending: 支払い画面を開いただけ。authorized: 仮押さえ済み。captured: 確定（PayPay は払った時点）。
    -- canceled: 仮押さえを取り消した。refunded: 確定後に返金した。
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'authorized', 'captured', 'canceled', 'refunded')),
    authorized_at timestamptz,
    captured_at timestamptz,
    canceled_at timestamptz,
    refunded_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (status = 'pending' OR payment_intent_id IS NOT NULL)
);

CREATE INDEX review_payments_request_idx ON review_payments (request_id);
-- 18歳未満の月の上限は、払った人のその月の合計で見る。
CREATE INDEX review_payments_payer_idx ON review_payments (payer_id, created_at);

ALTER TABLE review_payments ENABLE ROW LEVEL SECURITY;
