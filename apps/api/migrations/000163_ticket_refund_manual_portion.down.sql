-- needs_action の行が残っていると、旧制約を戻せずに失敗する。Stripe 外の返金の記録（額・方法・日付）は失われる。
ALTER TABLE coaching_ticket_refunds DROP CONSTRAINT coaching_ticket_refunds_status_check;
ALTER TABLE coaching_ticket_refunds ADD CONSTRAINT coaching_ticket_refunds_status_check
    CHECK (status IN ('pending', 'succeeded', 'failed'));
ALTER TABLE coaching_ticket_refunds
    DROP CONSTRAINT coaching_ticket_refunds_manual_done_check,
    DROP CONSTRAINT coaching_ticket_refunds_manual_yen_check,
    DROP COLUMN manual_completed_at,
    DROP COLUMN manual_paid_on,
    DROP COLUMN manual_method,
    DROP COLUMN manual_yen,
    DROP COLUMN post_expiry;
