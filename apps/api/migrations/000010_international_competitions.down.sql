DELETE FROM competitions WHERE prefecture = '00';

ALTER TABLE competitions DROP CONSTRAINT competitions_prefecture_chk;

ALTER TABLE competitions
    ADD CONSTRAINT competitions_prefecture_chk
    CHECK (prefecture ~ '^(0[1-9]|[1-3][0-9]|4[0-7])$');
