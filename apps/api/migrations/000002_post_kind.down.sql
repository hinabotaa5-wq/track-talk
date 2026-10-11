DROP INDEX IF EXISTS posts_youtube_video_id_active_idx;
ALTER TABLE posts DROP CONSTRAINT IF EXISTS posts_kind_fields;
ALTER TABLE posts DROP CONSTRAINT IF EXISTS posts_kind_check;
ALTER TABLE posts DROP COLUMN IF EXISTS cloudinary_public_id;
ALTER TABLE posts DROP COLUMN IF EXISTS youtube_thumbnail_url;
ALTER TABLE posts DROP COLUMN IF EXISTS youtube_video_id;
ALTER TABLE posts DROP COLUMN IF EXISTS kind;
