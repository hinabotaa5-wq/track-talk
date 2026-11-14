-- #704: 「目標の大会をセットする」機能の廃止に伴い、target_meets を落とす。
-- 既存ユーザーの設定データも一緒に消える（運営者決定の範囲内）。
DROP TABLE IF EXISTS target_meets;
