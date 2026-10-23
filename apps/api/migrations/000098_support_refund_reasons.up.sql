-- 大会サポートの未提供分返金の理由を、支払いの返金理由として記録できるようにする。
ALTER TABLE coaching_payments DROP CONSTRAINT coaching_payments_refund_reason_check;
ALTER TABLE coaching_payments ADD CONSTRAINT coaching_payments_refund_reason_check CHECK (refund_reason IN ('no_coach_reply', 'not_started', 'admin_full', 'admin_remaining', 'support_unprovided', 'support_admin_full', 'support_no_response'));
