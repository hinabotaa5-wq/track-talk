CREATE TABLE practice_ratings (
    id uuid PRIMARY KEY,
    post_id uuid NOT NULL REFERENCES posts (id) ON DELETE CASCADE,
    application_id uuid NOT NULL REFERENCES practice_applications (id) ON DELETE CASCADE,
    rater_id uuid NOT NULL REFERENCES profiles (id) ON DELETE RESTRICT,
    ratee_id uuid NOT NULL REFERENCES profiles (id) ON DELETE RESTRICT,
    score text NOT NULL CHECK (score IN ('good', 'ok', 'bad')),
    comment text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (application_id, rater_id),
    CHECK (rater_id <> ratee_id),
    CHECK (char_length(comment) <= 200)
);

CREATE INDEX practice_ratings_ratee_idx ON practice_ratings (ratee_id, created_at DESC);
