-- 部分返金の記録（partially_refunded・2件目以降の返金操作・一部返金の通知）があると戻せない。
-- Stripe 側の返金と会計の履歴はスキーマの巻き戻しでは消えないため、照合して前に進めること（000127／000128 と同じ考え方）。
DO $$ BEGIN
 PERFORM pg_advisory_xact_lock(hashtext('coach_payouts'));
 LOCK TABLE review_payments,coaching_consultations,captured_payment_refunds,refund_completion_notices IN SHARE ROW EXCLUSIVE MODE;
 IF EXISTS(SELECT 1 FROM review_payments WHERE status='partially_refunded')
  OR EXISTS(SELECT 1 FROM coaching_consultations WHERE payment_state='partially_refunded')
  OR EXISTS(SELECT 1 FROM captured_payment_refunds WHERE status<>'failed' GROUP BY source,payment_id HAVING count(*)>1)
  OR EXISTS(SELECT 1 FROM refund_completion_notices WHERE kind IN ('review_partial_refund','consultation_partial_refund')) THEN
  RAISE EXCEPTION 'partial refund accounting exists: rollback refused; reconcile and roll forward';
 END IF;
END $$;
DROP TRIGGER rebalance_refunded_new_earning ON coach_earnings;
DROP FUNCTION rebalance_refunded_new_earning();
DROP FUNCTION rebalance_captured_refund_earnings(text, uuid, uuid);
ALTER TABLE refund_completion_notices DROP CONSTRAINT refund_completion_notices_kind_check;
ALTER TABLE refund_completion_notices ADD CONSTRAINT refund_completion_notices_kind_check CHECK (kind IN ('review_cancel','review_refund','consultation_refund'));
ALTER TABLE refund_completion_notices DROP COLUMN amount_yen;

CREATE OR REPLACE FUNCTION link_review_payment_earning() RETURNS trigger LANGUAGE plpgsql SET search_path = 'public' AS $$
BEGIN
 IF NEW.source='review' THEN
 INSERT INTO review_payment_earning_links(payment_id,earning_id)
 SELECT p.id,NEW.id FROM review_payments p WHERE p.request_id=NEW.review_request_id AND p.status IN ('captured','refunded','refund_pending') ORDER BY CASE WHEN p.status='refunded' THEN 1 ELSE 0 END,p.authorized_at,p.id LIMIT 1 ON CONFLICT DO NOTHING;
 END IF;
 RETURN NEW;
END $$;

DROP INDEX captured_payment_refunds_payment_idx;
DROP INDEX captured_payment_refunds_active_admin_idx;
CREATE UNIQUE INDEX captured_payment_refunds_active_idx ON captured_payment_refunds(source,payment_id) WHERE status <> 'failed';

DROP TRIGGER fill_full_refunded_yen ON coaching_consultations;
DROP TRIGGER fill_full_refunded_yen ON review_payments;
DROP FUNCTION fill_full_refunded_yen();
ALTER TABLE coaching_consultations DROP CONSTRAINT coaching_consultations_refunded_yen_check;
ALTER TABLE review_payments DROP CONSTRAINT review_payments_refunded_yen_check;
ALTER TABLE coaching_consultations DROP CONSTRAINT coaching_consultations_payment_state_check;
ALTER TABLE coaching_consultations ADD CONSTRAINT coaching_consultations_payment_state_check CHECK (payment_state IN ('unpaid','paid','refund_pending','refunded'));
ALTER TABLE review_payments DROP CONSTRAINT review_payments_status_check;
ALTER TABLE review_payments ADD CONSTRAINT review_payments_status_check CHECK (status IN ('pending','authorized','captured','canceled','refund_pending','refunded'));
ALTER TABLE coaching_consultations DROP COLUMN refunded_yen;
ALTER TABLE review_payments DROP COLUMN refunded_yen;
