ALTER TABLE profiles DROP CONSTRAINT profiles_nickname_length_check;

ALTER TABLE profiles
    ALTER COLUMN nickname TYPE varchar(30);

ALTER TABLE profiles
    ADD CONSTRAINT profiles_nickname_length_check
    CHECK (
        nickname IS NULL
        OR char_length(btrim(nickname)) BETWEEN 1 AND 30
    );
