CREATE TABLE major_hosts (
    kind text NOT NULL CHECK (kind IN ('worlds', 'olympics')),
    year int NOT NULL CHECK (year BETWEEN 1896 AND 2100),
    name text NOT NULL,
    city text NOT NULL,
    country text NOT NULL DEFAULT '',
    venue text NOT NULL DEFAULT '',
    starts_on date NOT NULL,
    ends_on date NOT NULL,
    schedule_label text NOT NULL,
    image_url text NOT NULL DEFAULT '',
    page_url text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (kind, year)
);

ALTER TABLE major_hosts ENABLE ROW LEVEL SECURITY;
