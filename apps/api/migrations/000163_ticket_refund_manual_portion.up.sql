-- 失効後の返金（本体 #572 項目7）で、1つの決済への返金の合計が元の決済額を超える分は、
-- Stripe では返金できない（元の決済額が上限）。運営者決定（2026-11-12）により、Stripe へは返金できる残りの額までを返し、
-- 超えた分は運営者が Stripe の外（銀行振込など）で返す。その分の額と、支払った方法・日付を返金の行に残す。
-- amount_yen は返金の合計（Stripe 分＋Stripe 外の分）のまま。Stripe 分は amount_yen - manual_yen。
ALTER TABLE coaching_ticket_refunds
    ADD COLUMN post_expiry boolean NOT NULL DEFAULT false,
    ADD COLUMN manual_yen integer NOT NULL DEFAULT 0,
    ADD COLUMN manual_method text,
    ADD COLUMN manual_paid_on date,
    ADD COLUMN manual_completed_at timestamptz,
    ADD CONSTRAINT coaching_ticket_refunds_manual_yen_check CHECK (manual_yen >= 0 AND manual_yen <= amount_yen),
    ADD CONSTRAINT coaching_ticket_refunds_manual_done_check CHECK (
        (manual_completed_at IS NULL AND manual_method IS NULL AND manual_paid_on IS NULL)
        OR (manual_completed_at IS NOT NULL AND manual_method IS NOT NULL AND manual_paid_on IS NOT NULL AND manual_yen > 0));
-- Stripe が金額の超過で返金を断ったときは、再試行をやめて運営者の対応待ち（needs_action）にする。
ALTER TABLE coaching_ticket_refunds DROP CONSTRAINT coaching_ticket_refunds_status_check;
ALTER TABLE coaching_ticket_refunds ADD CONSTRAINT coaching_ticket_refunds_status_check
    CHECK (status IN ('pending', 'succeeded', 'failed', 'needs_action'));
