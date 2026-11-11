-- 県別年間スケジュール機能の廃止（#549）。県別大会の既存行を削除する。
-- 国際大会（00）は残し、今後の取り込み・表示の対象にする。
-- target_meets は名前と日付を写し取って持つため、県別行の削除で既存ユーザーの表示は壊れない。
DELETE FROM competitions WHERE prefecture <> '00';
-- 県別取り込みの試行の記録と、県陸協ニュースの確認時刻は、止めたジョブのものなので捨てる。
DROP TABLE IF EXISTS competition_import_attempts;
DROP TABLE IF EXISTS competition_news_checks;
