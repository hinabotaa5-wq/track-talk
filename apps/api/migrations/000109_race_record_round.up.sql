ALTER TABLE race_records
    ADD COLUMN round text NOT NULL DEFAULT '',
    ADD CONSTRAINT race_records_round_length CHECK (char_length(round) <= 20);
