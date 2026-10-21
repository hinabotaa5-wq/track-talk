-- injury_demo.sql で入れたケガのデモデータを消す。
-- デモ用プロフィール(00000000-0000-4000-8000-0000000000NN)を消すと、グループへの参加と段階の記録も一緒に消える。
-- 実行: make unseed-injury(ローカル)。本番は injury_demo.sql の先頭の手順を参照。
BEGIN;

DELETE FROM profiles WHERE id::text LIKE '00000000-0000-4000-8000-0000000000%';

COMMIT;
