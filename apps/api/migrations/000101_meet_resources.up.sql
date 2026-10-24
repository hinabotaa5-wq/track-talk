CREATE TABLE meet_resources (
    meet_key text PRIMARY KEY,
    official_url text NOT NULL DEFAULT '',
    timetable_url text NOT NULL DEFAULT '',
    youtube_video_id text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE meet_resources ENABLE ROW LEVEL SECURITY;
