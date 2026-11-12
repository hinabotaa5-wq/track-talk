-- 金庫 DB の keep-alive（H3）専用ロール（件数の読み取りのみ）。schema.sql を適用した移行オーナーで実行する。
-- パスワードはここに書かない。運営者が別途設定し、接続 URL（sslmode=require 以上）を
-- keep-alive ジョブ用の秘密に入れる。金庫の全権キーや鍵素材はこのロールに一切渡さない。
--
-- 輸出ロール（identity_export）と違い BYPASSRLS は付けない。件数の取得に行セキュリティの
-- 素通しは要らず、権限は最小（USAGE＋SELECT のみ、SEQUENCE 不要）にする。
-- 読み取り専用はロールの既定＋接続時の PGOPTIONS＋トランザクションの三重で守る。
-- この環境で読み取り専用にできない場合は、方式を変えずに止めて報告する。
CREATE ROLE identity_dbping LOGIN NOINHERIT;
ALTER ROLE identity_dbping SET default_transaction_read_only = on;
GRANT USAGE ON SCHEMA identity_vault TO identity_dbping;
GRANT SELECT ON ALL TABLES IN SCHEMA identity_vault TO identity_dbping;
-- 後から足すテーブルも読めるようにする（このファイルを実行したロールが作る物に効く）。
ALTER DEFAULT PRIVILEGES IN SCHEMA identity_vault GRANT SELECT ON TABLES TO identity_dbping;
