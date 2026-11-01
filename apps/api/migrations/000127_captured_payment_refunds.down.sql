-- External refunds and accounting history cannot be undone by schema rollback.
DO $$ BEGIN
 PERFORM pg_advisory_xact_lock(hashtext('coach_payouts'));
 LOCK TABLE captured_payment_refunds,captured_refund_earnings,coach_refund_adjustments,coach_refund_applications IN ACCESS EXCLUSIVE MODE;
 IF EXISTS(SELECT 1 FROM captured_payment_refunds) OR EXISTS(SELECT 1 FROM captured_refund_earnings) OR EXISTS(SELECT 1 FROM coach_refund_adjustments) OR EXISTS(SELECT 1 FROM coach_refund_applications) THEN
 RAISE EXCEPTION 'refund accounting exists: rollback refused; reconcile and roll forward';
 END IF;
END $$;
DROP TRIGGER restore_deleted_earning_offsets ON coach_earnings;
DROP FUNCTION restore_deleted_earning_offsets();
DROP TRIGGER link_review_payment_earning ON coach_earnings;
DROP FUNCTION link_review_payment_earning();
DROP TABLE review_payment_earning_links;
DROP TABLE coach_refund_applications, coach_refund_adjustments, captured_refund_earnings, captured_payment_refunds;
UPDATE review_payments SET status='captured' WHERE status='refund_pending';
ALTER TABLE review_payments DROP CONSTRAINT review_payments_status_check;
ALTER TABLE review_payments ADD CONSTRAINT review_payments_status_check CHECK (status IN ('pending','authorized','captured','canceled','refunded'));
