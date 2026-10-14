DROP TABLE IF EXISTS competition_news_checks;

ALTER TABLE competitions
    DROP COLUMN IF EXISTS guidelines_url;
