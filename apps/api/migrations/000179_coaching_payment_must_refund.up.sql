-- 期間を始めないと決めて全額返金する支払い（カードの発行国で止めたものなど）の印。
-- 返金がいったん Stripe で失敗しても、契約の状態にかかわらず tick が返金し終えるまで拾い続ける。
-- この支払いは契約の代金として扱わない（二重払いの判定・報酬・精算の対象から外す）。
ALTER TABLE coaching_payments ADD COLUMN must_refund boolean NOT NULL DEFAULT false;
