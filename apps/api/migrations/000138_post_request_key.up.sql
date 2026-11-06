-- 投稿の作成を冪等にする(#382)。Web は投稿の意図ごとに UUID を1つ作って送り、再試行では同じ値を使う。
-- 既存の投稿と、キーを送らない古いクライアントの投稿は NULL のまま(一意の対象外)。
-- 削除済みの投稿もキーを持ち続けるので、同じキーで作り直されることはない。
ALTER TABLE posts ADD COLUMN request_key uuid;
CREATE UNIQUE INDEX posts_author_request_key_idx ON posts (author_id, request_key) WHERE request_key IS NOT NULL;
