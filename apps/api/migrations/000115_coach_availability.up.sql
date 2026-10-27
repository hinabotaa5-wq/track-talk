CREATE TABLE coach_availability (
    coach_id uuid NOT NULL REFERENCES coach_profiles (profile_id) ON DELETE CASCADE,
    day date NOT NULL,
    status text NOT NULL CHECK (status IN ('available', 'consult', 'unavailable')),
    note text NOT NULL DEFAULT '' CHECK (char_length(note) <= 30),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (coach_id, day)
);
ALTER TABLE coach_availability ENABLE ROW LEVEL SECURITY;

ALTER TABLE coaching_contracts ADD COLUMN preferred_dates date[] NOT NULL DEFAULT '{}'
    CHECK (cardinality(preferred_dates) <= 3);
