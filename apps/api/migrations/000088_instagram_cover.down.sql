-- Instagram の表紙の画像を外してから、付けられなかったころの制約に戻す。
UPDATE posts SET cloudinary_public_id = NULL WHERE kind = 'instagram';

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
                    kind IN ('recruitment', 'practice', 'injury', 'meet')
                    AND youtube_video_id IS NULL
                    AND youtube_thumbnail_url IS NULL
                    AND cloudinary_public_id IS NULL
                )
            )
        )
    );
