-- 同じ写真・動画(cloudinary_public_id)を、削除されていない投稿のうち1件だけが使えるようにする(#382)。
-- 非表示の投稿も写真・動画を使い続けるので対象に含める。削除した投稿は対象外。
-- 本番に重複が無いことを確かめてから入れる。重複があるとこの migration は失敗する。
CREATE UNIQUE INDEX posts_cloudinary_public_id_active_idx ON posts (cloudinary_public_id)
    WHERE deleted_at IS NULL AND cloudinary_public_id IS NOT NULL;
