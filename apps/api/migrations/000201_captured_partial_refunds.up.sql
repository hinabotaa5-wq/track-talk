-- 本体 #1097: Stripe ダッシュボードの部分返金を、動画添削（review_payments）と追加質問（coaching_consultations）に同期する。
-- 支払いごとに返金済みの累計（refunded_yen）を持ち、一部だけ返金した状態を partially_refunded で表す。
ALTER TABLE review_payments ADD COLUMN refunded_yen integer NOT NULL DEFAULT 0;
ALTER TABLE coaching_consultations ADD COLUMN refunded_yen integer NOT NULL DEFAULT 0;
-- 既存の返金済みは全額返金済み、それ以外は 0 のまま。
UPDATE review_payments SET refunded_yen=amount_yen WHERE status='refunded';
UPDATE coaching_consultations SET refunded_yen=price_yen WHERE payment_state='refunded';

ALTER TABLE review_payments DROP CONSTRAINT review_payments_status_check;
ALTER TABLE review_payments ADD CONSTRAINT review_payments_status_check CHECK (status IN ('pending','authorized','captured','canceled','refund_pending','partially_refunded','refunded'));
ALTER TABLE coaching_consultations DROP CONSTRAINT coaching_consultations_payment_state_check;
ALTER TABLE coaching_consultations ADD CONSTRAINT coaching_consultations_payment_state_check CHECK (payment_state IN ('unpaid','paid','refund_pending','partially_refunded','refunded'));
ALTER TABLE review_payments ADD CONSTRAINT review_payments_refunded_yen_check CHECK (refunded_yen >= 0 AND refunded_yen <= amount_yen);
ALTER TABLE coaching_consultations ADD CONSTRAINT coaching_consultations_refunded_yen_check CHECK (refunded_yen >= 0 AND refunded_yen <= price_yen);

-- 返金済み（refunded）にする経路は captured_refund.go 以外にもある（期限切れの自動返金・PayPay の解放など）。
-- どの経路でも「refunded なら累計は全額」に揃える。
CREATE FUNCTION fill_full_refunded_yen() RETURNS trigger LANGUAGE plpgsql SET search_path = 'public' AS $$
BEGIN
 IF TG_TABLE_NAME = 'review_payments' THEN
  IF NEW.status = 'refunded' THEN NEW.refunded_yen := NEW.amount_yen; END IF;
 ELSIF NEW.payment_state = 'refunded' THEN
  NEW.refunded_yen := NEW.price_yen;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER fill_full_refunded_yen BEFORE INSERT OR UPDATE ON review_payments FOR EACH ROW EXECUTE FUNCTION fill_full_refunded_yen();
CREATE TRIGGER fill_full_refunded_yen BEFORE INSERT OR UPDATE ON coaching_consultations FOR EACH ROW EXECUTE FUNCTION fill_full_refunded_yen();

-- 1つの支払いに部分返金が複数ありうる。管理画面からの返金（全額か残額）だけは、支払いごとに進行中1件のまま。
DROP INDEX captured_payment_refunds_active_idx;
CREATE UNIQUE INDEX captured_payment_refunds_active_admin_idx ON captured_payment_refunds(source,payment_id) WHERE status <> 'failed' AND origin = 'admin';
CREATE INDEX captured_payment_refunds_payment_idx ON captured_payment_refunds(source,payment_id);

-- 一部返金のあとに報酬を記録した添削も、支払いと報酬を結びつける。
CREATE OR REPLACE FUNCTION link_review_payment_earning() RETURNS trigger LANGUAGE plpgsql SET search_path = 'public' AS $$
BEGIN
 IF NEW.source='review' THEN
 INSERT INTO review_payment_earning_links(payment_id,earning_id)
 SELECT p.id,NEW.id FROM review_payments p WHERE p.request_id=NEW.review_request_id AND p.status IN ('captured','partially_refunded','refunded','refund_pending') ORDER BY CASE WHEN p.status='refunded' THEN 1 ELSE 0 END,p.authorized_at,p.id LIMIT 1 ON CONFLICT DO NOTHING;
 END IF;
 RETURN NEW;
END $$;

-- 一部返金の通知は、返金した金額を本文に出す。
ALTER TABLE refund_completion_notices ADD COLUMN amount_yen integer CHECK (amount_yen > 0);
ALTER TABLE refund_completion_notices DROP CONSTRAINT refund_completion_notices_kind_check;
ALTER TABLE refund_completion_notices ADD CONSTRAINT refund_completion_notices_kind_check CHECK (kind IN ('review_cancel','review_refund','consultation_refund','review_partial_refund','consultation_partial_refund'));

-- 部分返金のコーチ報酬の減額（本体 #1097 §2-3）。
-- 返金累計に対する減額の目標額は floor(報酬額 × 返金累計 ÷ 支払額)。既に減額済みの額（成功した返金操作の相殺台帳）との差分だけを
-- 相殺台帳（coach_refund_adjustments）に足す。送金済みでも未送金でも相殺台帳で減らし、過去の送金は書き換えない。
-- 累計が全額に達した時点で減額の総額は報酬額ちょうどになる。全額を1回で返金した操作があるときは、従来の全額の確定処理
-- （未送金は除外、送金済みは相殺）が報酬を持つので、ここでは何もしない。
-- p_anchor は差分を載せる返金操作（成功済み）。NULL なら、その支払いで最後に成功した返金操作に載せる。
CREATE FUNCTION rebalance_captured_refund_earnings(p_source text, p_payment uuid, p_anchor uuid) RETURNS void LANGUAGE plpgsql SET search_path = 'public' AS $$
DECLARE
 total integer; cum integer; anchor uuid := p_anchor; e record; a record; target integer; net integer; delta integer; n integer;
BEGIN
 IF p_source = 'review' THEN
  SELECT amount_yen INTO total FROM review_payments WHERE id = p_payment;
 ELSIF p_source = 'consultation' THEN
  SELECT price_yen INTO total FROM coaching_consultations WHERE id = p_payment;
 END IF;
 IF total IS NULL OR total <= 0 THEN RETURN; END IF;
 IF EXISTS(SELECT 1 FROM captured_payment_refunds WHERE source = p_source AND payment_id = p_payment AND status = 'succeeded' AND amount_yen >= total) THEN
  RETURN;
 END IF;
 SELECT COALESCE(sum(amount_yen), 0)::integer INTO cum FROM captured_payment_refunds WHERE source = p_source AND payment_id = p_payment AND status = 'succeeded';
 IF anchor IS NULL THEN
  SELECT id INTO anchor FROM captured_payment_refunds WHERE source = p_source AND payment_id = p_payment AND status = 'succeeded'
  ORDER BY succeeded_at DESC NULLS LAST, created_at DESC, id DESC LIMIT 1;
 END IF;
 FOR e IN SELECT ce.id, ce.coach_id, ce.amount_yen FROM coach_earnings ce WHERE
  (p_source = 'review' AND ce.source = 'review' AND ce.id IN (SELECT earning_id FROM review_payment_earning_links WHERE payment_id = p_payment)) OR
  (p_source = 'consultation' AND ce.source = 'consultation' AND ce.consultation_id = p_payment)
  ORDER BY ce.id FOR UPDATE
 LOOP
  target := (e.amount_yen::bigint * cum / total)::integer;
  SELECT COALESCE(sum(x.amount_yen), 0)::integer INTO net FROM coach_refund_adjustments x JOIN captured_payment_refunds f ON f.id = x.refund_id
  WHERE x.earning_id = e.id AND x.kind = 'refund_offset' AND f.status = 'succeeded' AND f.source = p_source AND f.payment_id = p_payment;
  delta := target - net;
  IF delta > 0 AND anchor IS NOT NULL THEN
   INSERT INTO coach_refund_adjustments(id, refund_id, earning_id, coach_id, amount_yen) VALUES (gen_random_uuid(), anchor, e.id, e.coach_id, delta)
   ON CONFLICT ON CONSTRAINT coach_refund_adjustments_operation_earning_key DO UPDATE SET amount_yen = coach_refund_adjustments.amount_yen + EXCLUDED.amount_yen;
  ELSIF delta < 0 THEN
   -- 失敗した返金の取り消しの端数で減らしすぎたときは、まだ適用していない相殺だけを減らす。適用済みの履歴は書き換えない。
   FOR a IN SELECT x.id, x.amount_yen, x.applied_yen FROM coach_refund_adjustments x JOIN captured_payment_refunds f ON f.id = x.refund_id
    WHERE x.earning_id = e.id AND x.kind = 'refund_offset' AND f.status = 'succeeded' AND f.source = p_source AND f.payment_id = p_payment
    AND NOT EXISTS(SELECT 1 FROM coach_refund_cancellations c WHERE c.adjustment_id = x.id)
    ORDER BY x.created_at DESC, x.id DESC FOR UPDATE OF x
   LOOP
    EXIT WHEN delta = 0;
    n := LEAST(-delta, a.amount_yen - GREATEST(a.applied_yen, 1));
    IF n > 0 THEN
     UPDATE coach_refund_adjustments SET amount_yen = amount_yen - n WHERE id = a.id;
     delta := delta + n;
    END IF;
   END LOOP;
  END IF;
 END LOOP;
END $$;

-- 一部返金のあとで記録した報酬（返却後にまとめて記録する添削、回答時に記録する追加質問）も、その時点の累計で減額する。
-- 同じ AFTER INSERT の link_review_payment_earning（支払いとの結びつけ）より後に動くよう、名前の順を後ろにしている。
CREATE FUNCTION rebalance_refunded_new_earning() RETURNS trigger LANGUAGE plpgsql SET search_path = 'public' AS $$
DECLARE src text; pid uuid;
BEGIN
 IF NEW.source = 'review' THEN
  src := 'review';
  SELECT payment_id INTO pid FROM review_payment_earning_links WHERE earning_id = NEW.id;
 ELSIF NEW.source = 'consultation' THEN
  src := 'consultation';
  pid := NEW.consultation_id;
 END IF;
 IF pid IS NOT NULL AND EXISTS(SELECT 1 FROM captured_payment_refunds WHERE source = src AND payment_id = pid AND status = 'succeeded') THEN
  PERFORM rebalance_captured_refund_earnings(src, pid, NULL);
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER rebalance_refunded_new_earning AFTER INSERT ON coach_earnings FOR EACH ROW EXECUTE FUNCTION rebalance_refunded_new_earning();
