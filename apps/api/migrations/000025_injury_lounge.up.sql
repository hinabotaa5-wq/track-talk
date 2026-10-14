ALTER TABLE posts DROP CONSTRAINT posts_kind_check;
ALTER TABLE posts
    ADD CONSTRAINT posts_kind_check CHECK (kind IN ('text', 'youtube', 'video', 'recruitment', 'practice', 'injury'));

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
            kind IN ('recruitment', 'practice', 'injury')
            AND youtube_video_id IS NULL
            AND youtube_thumbnail_url IS NULL
            AND cloudinary_public_id IS NULL
        )
    );

CREATE TABLE injury_details (
    post_id uuid PRIMARY KEY REFERENCES posts (id) ON DELETE CASCADE,
    mood text NOT NULL,
    body_part text NOT NULL,
    CHECK (mood IN ('vent', 'rehab')),
    CHECK (
        body_part IN (
            'foot',
            'ankle',
            'achilles',
            'calf',
            'shin',
            'knee',
            'quad',
            'hamstring',
            'groin',
            'hip',
            'lower-back',
            'upper-body',
            'other'
        )
    )
);

CREATE INDEX injury_details_mood_idx ON injury_details (mood, body_part);

ALTER TABLE injury_details ENABLE ROW LEVEL SECURITY;
