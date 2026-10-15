ALTER TABLE posts
    ADD COLUMN instagram_type text,
    ADD COLUMN instagram_shortcode text,
    ADD CONSTRAINT posts_instagram_type_check CHECK (instagram_type IN ('p', 'reel')),
    ADD CONSTRAINT posts_instagram_shortcode_check CHECK (instagram_shortcode ~ '^[A-Za-z0-9_-]{5,64}$');

ALTER TABLE posts DROP CONSTRAINT posts_kind_check;
ALTER TABLE posts
    ADD CONSTRAINT posts_kind_check CHECK (kind IN ('text', 'youtube', 'instagram', 'video', 'recruitment', 'practice', 'injury'));

ALTER TABLE posts DROP CONSTRAINT posts_kind_fields;
ALTER TABLE posts
    ADD CONSTRAINT posts_kind_fields CHECK (
        (
            kind = 'instagram'
            AND instagram_type IS NOT NULL
            AND instagram_shortcode IS NOT NULL
            AND youtube_video_id IS NULL
            AND youtube_thumbnail_url IS NULL
            AND cloudinary_public_id IS NULL
        )
        OR (
            kind <> 'instagram'
            AND instagram_type IS NULL
            AND instagram_shortcode IS NULL
            AND (
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
                    kind IN ('recruitment', 'practice', 'injury')
                    AND youtube_video_id IS NULL
                    AND youtube_thumbnail_url IS NULL
                    AND cloudinary_public_id IS NULL
                )
            )
        )
    );

CREATE UNIQUE INDEX posts_instagram_shortcode_active_idx
    ON posts (instagram_shortcode)
    WHERE deleted_at IS NULL AND hidden_at IS NULL AND instagram_shortcode IS NOT NULL;
