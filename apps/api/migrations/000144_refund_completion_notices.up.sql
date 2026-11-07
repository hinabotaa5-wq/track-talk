CREATE TABLE refund_completion_notices (
    id uuid PRIMARY KEY,
    recipient_id uuid NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('review_cancel', 'review_refund', 'consultation_refund')),
    url text NOT NULL,
    -- 保留中の自動返金は、Stripe から確定が届くまで送信を待つ。
    pending_stripe_refund_id text UNIQUE,
    next_attempt_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE refund_completion_notices ENABLE ROW LEVEL SECURITY;
CREATE INDEX refund_completion_notices_due_idx ON refund_completion_notices (next_attempt_at, id)
    WHERE pending_stripe_refund_id IS NULL;
