ALTER TABLE posts DROP CONSTRAINT posts_kind_check;
ALTER TABLE posts
    ADD CONSTRAINT posts_kind_check CHECK (kind IN ('text', 'youtube', 'video', 'recruitment'));

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
    );

CREATE TABLE recruitment_details (
    post_id uuid PRIMARY KEY REFERENCES posts (id) ON DELETE CASCADE,
    venue text NOT NULL DEFAULT '',
    schedule text NOT NULL,
    gender_ratio text NOT NULL DEFAULT '',
    average_age text NOT NULL DEFAULT '',
    recruit_age text NOT NULL DEFAULT ''
);

CREATE TABLE recruitment_prefectures (
    post_id uuid NOT NULL REFERENCES posts (id) ON DELETE CASCADE,
    position int NOT NULL,
    prefecture text NOT NULL,
    PRIMARY KEY (post_id, position),
    UNIQUE (post_id, prefecture),
    CHECK (position >= 0 AND position < 47),
    CHECK (prefecture ~ '^(0[1-9]|[1-3][0-9]|4[0-7])$')
);

CREATE TABLE recruitment_links (
    post_id uuid NOT NULL REFERENCES posts (id) ON DELETE CASCADE,
    position int NOT NULL,
    url text NOT NULL,
    PRIMARY KEY (post_id, position),
    CHECK (position >= 0 AND position < 20)
);

CREATE TABLE recruitment_photos (
    post_id uuid NOT NULL REFERENCES posts (id) ON DELETE CASCADE,
    position int NOT NULL,
    cloudinary_public_id text NOT NULL,
    PRIMARY KEY (post_id, position),
    CHECK (position >= 0 AND position < 3)
);
