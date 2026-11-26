-- 登録時に「生まれた年」だけを聞く（本体 #1045）。年齢層の把握と、購入前チェック
-- 「未成年の方は保護者に許可を取ってください」を未成年（の可能性がある人）だけに出すために使う。
-- 月・日は聞かない。既存の birth_month（サポートプランの年齢確認、月精度）とは別の列にし、
-- birth_month から導出した年をここへ書き戻すことはしない。一度登録したら本人は変えられない（API で 409）。
ALTER TABLE profiles
    ADD COLUMN birth_year integer;

ALTER TABLE profiles
    ADD CONSTRAINT profiles_birth_year_range CHECK (birth_year IS NULL OR birth_year BETWEEN 1900 AND 2100);
