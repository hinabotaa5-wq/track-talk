-- 管理画面の通報一覧は新しい順に読む。問い合わせ側とそろえて索引を付ける。
CREATE INDEX reports_created_idx ON reports (created_at DESC);
