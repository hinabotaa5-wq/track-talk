ALTER TABLE posts DROP CONSTRAINT posts_kind_fields;

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
