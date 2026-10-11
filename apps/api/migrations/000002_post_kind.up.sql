ALTER TABLE posts
    ADD COLUMN kind text NOT NULL DEFAULT 'text',
    ADD COLUMN youtube_video_id text,
    ADD COLUMN youtube_thumbnail_url text,
    ADD COLUMN cloudinary_public_id text;

ALTER TABLE posts
    ADD CONSTRAINT posts_kind_check CHECK (kind IN ('text', 'youtube', 'video'));

ALTER TABLE posts
    ADD CONSTRAINT posts_kind_fields CHECK (
        (
            kind = 'text'
            AND youtube_video_id IS NULL
            AND youtube_thumbnail_url IS NULL
            AND cloudinary_public_id IS NULL
        )
        OR (
            kind = 'youtube'
            AND youtube_video_id IS NOT NULL
            AND youtube_thumbnail_url IS NOT NULL
            AND cloudinary_public_id IS NULL
        )
        OR (
            kind = 'video'
            AND cloudinary_public_id IS NOT NULL
            AND youtube_video_id IS NULL
            AND youtube_thumbnail_url IS NULL
        )
    );

CREATE UNIQUE INDEX posts_youtube_video_id_active_idx
    ON posts (youtube_video_id)
    WHERE deleted_at IS NULL AND youtube_video_id IS NOT NULL;
