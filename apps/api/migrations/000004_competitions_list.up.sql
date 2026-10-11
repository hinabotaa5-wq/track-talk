-- Placeholder rows have no prefecture or official page, so they cannot satisfy the new columns.
DELETE FROM competitions;

DROP INDEX IF EXISTS competitions_starts_on_idx;

ALTER TABLE competitions
    DROP COLUMN scope,
    DROP COLUMN summary,
    ADD COLUMN prefecture text NOT NULL,
    ADD COLUMN season_year int NOT NULL,
    ADD COLUMN organizer_url text NOT NULL,
    ADD COLUMN source_url text NOT NULL,
    ADD CONSTRAINT competitions_prefecture_chk CHECK (prefecture ~ '^(0[1-9]|[1-3][0-9]|4[0-7])$'),
    ADD CONSTRAINT competitions_season_year_chk CHECK (season_year BETWEEN 1990 AND 2100);

CREATE UNIQUE INDEX competitions_identity_idx
    ON competitions (prefecture, season_year, name, starts_on);

CREATE INDEX competitions_prefecture_starts_on_idx
    ON competitions (prefecture, starts_on);
