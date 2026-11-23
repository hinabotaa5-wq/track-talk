-- 動画添削の価格改定（本体 #983、運営者決定 2026-11-23）。
-- 単発は1回1,000円（報酬は70%の700円）、チケットは6枚つづり5,000円（「5枚分で6枚」の形は維持）。
-- 単発の価格は API の reviewPriceYen が正本で、依頼作成時に base_price_yen へ書き込む。既定値もそれに揃える。
-- 既存の購入・ロット・予約・依頼の金額は書き換えない（チケットは未販売。運営者明言）。

ALTER TABLE review_requests ALTER COLUMN base_price_yen SET DEFAULT 1000;

UPDATE coaching_ticket_products SET amount_yen = 5000 WHERE id = 'six';
