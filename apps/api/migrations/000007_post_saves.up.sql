CREATE TABLE post_saves (
    post_id uuid NOT NULL REFERENCES posts (id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (post_id, user_id)
);

CREATE INDEX post_saves_user_created_idx ON post_saves (user_id, created_at DESC);

ALTER TABLE post_saves ENABLE ROW LEVEL SECURITY;
