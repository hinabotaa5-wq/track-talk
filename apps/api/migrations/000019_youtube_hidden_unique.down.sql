DROP INDEX IF EXISTS posts_youtube_video_id_active_idx;

CREATE UNIQUE INDEX posts_youtube_video_id_active_idx
    ON posts (youtube_video_id)
    WHERE deleted_at IS NULL AND youtube_video_id IS NOT NULL;
