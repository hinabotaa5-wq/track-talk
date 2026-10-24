-- 日時・いいね数が同じ投稿もIDで順序を確定する。ホームでは非公開グループを走査しない。
CREATE INDEX posts_home_new_cursor_idx ON posts (created_at DESC, id DESC)
WHERE deleted_at IS NULL AND hidden_at IS NULL AND kind NOT IN ('injury', 'meet');
CREATE INDEX posts_home_popular_cursor_idx ON posts (like_count DESC, created_at DESC, id DESC)
WHERE deleted_at IS NULL AND hidden_at IS NULL AND kind NOT IN ('injury', 'meet');
