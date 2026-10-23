-- メッセージ保存期間後も、精算済みの提供数は会計情報として残す。
ALTER TABLE coaching_contracts ADD COLUMN delivered_video_count integer NOT NULL DEFAULT 0,
 ADD COLUMN delivered_call_count integer NOT NULL DEFAULT 0;
