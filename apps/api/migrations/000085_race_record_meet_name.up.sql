-- 記録を出した大会の名前。任意で、書かなければ空。
ALTER TABLE race_records
    ADD COLUMN meet_name text NOT NULL DEFAULT '',
    ADD CONSTRAINT race_records_meet_name_length CHECK (char_length(meet_name) <= 60);
