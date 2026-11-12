-- チケットの有効期限のリマインド（本体 #569 フェーズB）。失効の30日前と7日前に1回ずつ知らせる。
-- 送ったかどうかをロットごとに残し、同じ知らせを2回送らない。
ALTER TABLE coaching_ticket_lots
    ADD COLUMN reminded_30_at timestamptz,
    ADD COLUMN reminded_7_at timestamptz;
