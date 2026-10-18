ALTER TABLE menu_contest_entries DROP CONSTRAINT menu_contest_entries_body_check;
ALTER TABLE menu_contest_entries
    ADD CONSTRAINT menu_contest_entries_body_check CHECK (char_length(btrim(body)) BETWEEN 1 AND 200);
