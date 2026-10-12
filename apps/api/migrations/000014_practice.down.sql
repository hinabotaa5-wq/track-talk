DELETE FROM posts WHERE kind = 'practice';

DROP TABLE IF EXISTS practice_details;

ALTER TABLE posts DROP CONSTRAINT posts_kind_fields;
ALTER TABLE posts DROP CONSTRAINT posts_kind_check;

ALTER TABLE posts
    ADD CONSTRAINT posts_kind_check CHECK (kind IN ('text', 'youtube', 'video', 'recruitment'));

ALTER TABLE posts
    ADD CONSTRAINT posts_kind_fields CHECK (
        (
            kind = 'text'
            AND youtube_video_id IS NULL
            AND youtube_thumbnail_url IS NULL
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
        OR (
            kind = 'recruitment'
            AND youtube_video_id IS NULL
            AND youtube_thumbnail_url IS NULL
            AND cloudinary_public_id IS NULL
        )
    );
