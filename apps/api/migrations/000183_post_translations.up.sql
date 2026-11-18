CREATE TABLE post_translations (
    post_id uuid PRIMARY KEY REFERENCES posts(id) ON DELETE CASCADE,
    source_hash text NOT NULL,
    title_en text NOT NULL DEFAULT '',
    body_en text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE post_translations ENABLE ROW LEVEL SECURITY;
CREATE TABLE post_translation_attempts (
    id bigserial PRIMARY KEY,
    attempted_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX post_translation_attempts_attempted_at_idx ON post_translation_attempts (attempted_at);
ALTER TABLE post_translation_attempts ENABLE ROW LEVEL SECURITY;
