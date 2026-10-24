ALTER TABLE coach_profiles ADD COLUMN name text NOT NULL DEFAULT '' CHECK (char_length(name) <= 80);
