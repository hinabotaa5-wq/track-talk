ALTER TABLE practice_details
    ADD COLUMN dissolved_at timestamptz;

CREATE TABLE practice_applications (
    id uuid PRIMARY KEY,
    post_id uuid NOT NULL REFERENCES posts (id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES profiles (id) ON DELETE RESTRICT,
    status text NOT NULL CHECK (status IN ('pending', 'accepted', 'declined')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (post_id, user_id)
);

CREATE INDEX practice_applications_post_status_idx
    ON practice_applications (post_id, status);
