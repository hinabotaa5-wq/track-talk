CREATE TABLE injury_updates (
    id uuid PRIMARY KEY,
    post_id uuid NOT NULL REFERENCES posts (id) ON DELETE CASCADE,
    stage text NOT NULL,
    body text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (stage IN ('resting', 'rehab', 'jogging', 'practice', 'race')),
    CHECK (char_length(btrim(body)) BETWEEN 1 AND 2000)
);

CREATE INDEX injury_updates_post_created_idx ON injury_updates (post_id, created_at);

ALTER TABLE injury_updates ENABLE ROW LEVEL SECURITY;
