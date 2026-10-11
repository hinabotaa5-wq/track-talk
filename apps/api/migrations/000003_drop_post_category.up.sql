DROP INDEX IF EXISTS posts_competition_idx;
DROP INDEX IF EXISTS posts_category_idx;
ALTER TABLE posts DROP COLUMN IF EXISTS competition_id;
ALTER TABLE posts DROP COLUMN IF EXISTS category_id;
DROP TABLE IF EXISTS post_categories;
