ALTER TABLE posts DROP CONSTRAINT posts_kind_check;
ALTER TABLE posts
    ADD CONSTRAINT posts_kind_check CHECK (kind IN ('text', 'youtube', 'video', 'recruitment', 'practice'));

ALTER TABLE posts DROP CONSTRAINT posts_kind_fields;
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
        OR (
            kind = 'practice'
            AND youtube_video_id IS NULL
            AND youtube_thumbnail_url IS NULL
            AND cloudinary_public_id IS NULL
        )
    );

CREATE TABLE practice_details (
    post_id uuid PRIMARY KEY REFERENCES posts (id) ON DELETE CASCADE,
    venue text NOT NULL,
    starts_at timestamptz NOT NULL,
    event text NOT NULL,
    event_other text NOT NULL DEFAULT '',
    capacity int NOT NULL,
    CHECK (char_length(venue) BETWEEN 1 AND 80),
    CHECK (capacity BETWEEN 1 AND 99),
    CHECK (
        event IN (
            '100m',
            '200m',
            '400m',
            '800m',
            '1500m',
            '5000m',
            '10000m',
            '100mh',
            '110mh',
            '400mh',
            '3000msc',
            'long-jump',
            'triple-jump',
            'high-jump',
            'pole-vault',
            'shot-put',
            'discus',
            'hammer',
            'javelin',
            'heptathlon',
            'decathlon',
            '4x100',
            '4x400',
            'other'
        )
    ),
    CHECK (
        (
            event = 'other'
            AND char_length(event_other) BETWEEN 1 AND 40
        )
        OR (
            event <> 'other'
            AND event_other = ''
        )
    )
);
