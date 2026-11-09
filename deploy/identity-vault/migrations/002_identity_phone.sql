-- #440: (2) サポートプランのコーチの電話番号を、氏名・住所と同じく
-- フィールド暗号化して身元情報に持たせる。
--
-- schema.sql（H1 で適用済み）の後に、金庫の migration owner で適用する。
-- 列の追加だけで、既存の行・権限・RLS には触れない。何度流しても同じ結果になる。
-- 既存の行は電話番号なし（NULL）のまま残り、本人が登録し直すまで
-- 表記要のプランは公開されない（専用サービス側で判定する）。
ALTER TABLE identity_vault.identities ADD COLUMN IF NOT EXISTS phone_enc bytea;
