ALTER TABLE profiles
    DROP CONSTRAINT profiles_nickname_length_check,
    DROP COLUMN nickname_public,
    DROP COLUMN nickname;
