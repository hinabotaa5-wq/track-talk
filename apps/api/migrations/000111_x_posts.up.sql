ALTER TABLE posts ADD COLUMN x_post_id text;
ALTER TABLE posts DROP CONSTRAINT posts_kind_check;
ALTER TABLE posts ADD CONSTRAINT posts_kind_check CHECK (kind IN ('text', 'youtube', 'instagram', 'x', 'video', 'recruitment', 'practice', 'injury', 'meet'));
ALTER TABLE posts ADD CONSTRAINT posts_x_fields CHECK (
    (kind = 'x' AND x_post_id IS NOT NULL AND x_post_id ~ '^[0-9]{1,25}$') OR
    (kind <> 'x' AND x_post_id IS NULL)
);
CREATE UNIQUE INDEX posts_x_active_idx ON posts (x_post_id) WHERE deleted_at IS NULL AND hidden_at IS NULL AND x_post_id IS NOT NULL;
ALTER TABLE posts DROP CONSTRAINT posts_kind_fields;
ALTER TABLE posts
    ADD CONSTRAINT posts_kind_fields CHECK (
        (
            kind = 'instagram'
            AND instagram_type IS NOT NULL
            AND instagram_shortcode IS NOT NULL
            AND youtube_video_id IS NULL
            AND youtube_thumbnail_url IS NULL
        )
        OR (
            kind <> 'instagram'
            AND instagram_type IS NULL
            AND instagram_shortcode IS NULL
            AND (
                (
                    kind IN ('text', 'x')
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
                    kind IN ('recruitment', 'practice', 'injury', 'meet')
                    AND youtube_video_id IS NULL
                    AND youtube_thumbnail_url IS NULL
                    AND cloudinary_public_id IS NULL
                )
            )
        )
    );
