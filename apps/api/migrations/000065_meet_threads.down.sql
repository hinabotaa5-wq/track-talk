-- 実況スレッドを巻き戻す。制約を戻す前に 'meet' の投稿を消す(残っていると CHECK 違反になる)。
DROP TABLE IF EXISTS meet_threads;

DELETE FROM posts WHERE author_id = 'f47ac10b-58cc-4372-a567-0e02b2c3d479';

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

ALTER TABLE posts DROP CONSTRAINT posts_kind_check;
ALTER TABLE posts
    ADD CONSTRAINT posts_kind_check CHECK (kind IN ('text', 'youtube', 'instagram', 'video', 'recruitment', 'practice', 'injury'));

DELETE FROM profiles WHERE id = 'f47ac10b-58cc-4372-a567-0e02b2c3d479';
