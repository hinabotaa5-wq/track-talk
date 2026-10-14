ALTER TABLE profiles
    ADD COLUMN nickname varchar(30),
    ADD COLUMN nickname_public boolean NOT NULL DEFAULT false;

ALTER TABLE profiles
    ADD CONSTRAINT profiles_nickname_length_check
    CHECK (
        nickname IS NULL
        OR char_length(btrim(nickname)) BETWEEN 1 AND 30
    );
