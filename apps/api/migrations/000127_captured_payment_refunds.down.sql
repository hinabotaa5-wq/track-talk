DROP TRIGGER restore_deleted_earning_offsets ON coach_earnings;
DROP FUNCTION restore_deleted_earning_offsets();
DROP TRIGGER link_review_payment_earning ON coach_earnings;
DROP FUNCTION link_review_payment_earning();
DROP TABLE review_payment_earning_links;
DROP TABLE coach_refund_applications, coach_refund_adjustments, captured_refund_earnings, captured_payment_refunds;
UPDATE review_payments SET status='captured' WHERE status='refund_pending';
ALTER TABLE review_payments DROP CONSTRAINT review_payments_status_check;
ALTER TABLE review_payments ADD CONSTRAINT review_payments_status_check CHECK (status IN ('pending','authorized','captured','canceled','refunded'));
