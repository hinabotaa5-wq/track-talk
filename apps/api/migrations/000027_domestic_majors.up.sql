CREATE TABLE domestic_majors (
    slug text NOT NULL,
    year int NOT NULL CHECK (year BETWEEN 2000 AND 2100),
    name text NOT NULL,
    category text NOT NULL CHECK (category IN ('youth', 'student', 'open')),
    venue text NOT NULL DEFAULT '',
    starts_on date NOT NULL,
    ends_on date NOT NULL,
    schedule_label text NOT NULL,
    site_url text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (slug, year)
);

ALTER TABLE domestic_majors ENABLE ROW LEVEL SECURITY;
