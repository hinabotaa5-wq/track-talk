ALTER TABLE recruitment_links DROP CONSTRAINT IF EXISTS recruitment_links_position_check;

ALTER TABLE recruitment_links
    ADD CONSTRAINT recruitment_links_position_check CHECK (position >= 0 AND position < 20);
