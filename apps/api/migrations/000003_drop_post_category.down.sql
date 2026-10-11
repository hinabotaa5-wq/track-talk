CREATE TABLE IF NOT EXISTS post_categories (
    id text PRIMARY KEY,
    name text NOT NULL,
    sort_order int NOT NULL
);

ALTER TABLE posts ADD COLUMN IF NOT EXISTS category_id text REFERENCES post_categories (id);
ALTER TABLE posts ADD COLUMN IF NOT EXISTS competition_id uuid REFERENCES competitions (id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS posts_competition_idx ON posts (competition_id, created_at DESC)
    WHERE deleted_at IS NULL AND hidden_at IS NULL;
CREATE INDEX IF NOT EXISTS posts_category_idx ON posts (category_id, created_at DESC)
    WHERE deleted_at IS NULL AND hidden_at IS NULL;
