-- 注意：この down は、コーチの有効価格と価格申請の履歴を消す。消した値は戻せない。
DROP INDEX IF EXISTS review_requests_requested_coach_open_idx;
DROP TABLE coach_price_applications;
DROP TABLE coach_prices;
