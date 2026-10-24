-- 新しい購入だけに適用する。既存の購入・予約は保存済みの金額を維持する。
UPDATE coaching_ticket_products SET amount_yen = 600 WHERE id = 'single';
UPDATE coaching_ticket_products SET amount_yen = 2880 WHERE id = 'five';
