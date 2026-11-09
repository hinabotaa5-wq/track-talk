-- 金庫 DB の輸出専用ロール（読み取りのみ）。schema.sql を適用した移行オーナーで実行する。
-- パスワードはここに書かない。運営者が別途設定し、接続 URL（sslmode=require 以上）を
-- 輸出ジョブ用の秘密に入れる。金庫の全権キーや鍵素材はこのロールに一切渡さない。
--
-- 行セキュリティを素通しする BYPASSRLS を付ける。輸出は row_security=off で読むため、
-- 行セキュリティで行が隠れるロールでは空の輸出にならず、失敗として止まる。
-- この環境で BYPASSRLS を付けられない場合は、方式を変えずに止めて報告する。
CREATE ROLE identity_export LOGIN BYPASSRLS NOINHERIT;
ALTER ROLE identity_export SET default_transaction_read_only = on;
GRANT USAGE ON SCHEMA identity_vault TO identity_export;
GRANT SELECT ON ALL TABLES IN SCHEMA identity_vault TO identity_export;
GRANT SELECT ON ALL SEQUENCES IN SCHEMA identity_vault TO identity_export;
-- 後から足すテーブルも読めるようにする（このファイルを実行したロールが作る物に効く）。
ALTER DEFAULT PRIVILEGES IN SCHEMA identity_vault GRANT SELECT ON TABLES TO identity_export;
ALTER DEFAULT PRIVILEGES IN SCHEMA identity_vault GRANT SELECT ON SEQUENCES TO identity_export;
