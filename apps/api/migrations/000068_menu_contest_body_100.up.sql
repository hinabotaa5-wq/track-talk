-- エグ練大会の応募を100文字までにする。
-- すでに200文字までで出された応募は残すので、既存の行は検査しない(NOT VALID)。
ALTER TABLE menu_contest_entries DROP CONSTRAINT menu_contest_entries_body_check;
ALTER TABLE menu_contest_entries
    ADD CONSTRAINT menu_contest_entries_body_check CHECK (char_length(btrim(body)) BETWEEN 1 AND 100) NOT VALID;
