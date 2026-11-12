-- チケット条項の改訂（本体 #572 v3）。購入のときに表示していた文面の版を購入ごとに残す（証拠化）。
ALTER TABLE coaching_ticket_purchases ADD COLUMN wording_version text;
