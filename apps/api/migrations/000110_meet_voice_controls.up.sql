CREATE TABLE meet_voice_settings (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    paused boolean NOT NULL DEFAULT false,
    last_tick_at timestamptz,
    restrict_to_posts boolean NOT NULL DEFAULT true,
    allowed_post_ids uuid[] NOT NULL DEFAULT '{}',
    monthly_minutes_limit integer NOT NULL DEFAULT 4000 CHECK (monthly_minutes_limit BETWEEN 1 AND 100000)
);
ALTER TABLE meet_voice_settings ENABLE ROW LEVEL SECURITY;
INSERT INTO meet_voice_settings(singleton) VALUES(true);
ALTER TABLE meet_voice_reservations ADD COLUMN last_seen_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN confirmed_at timestamptz NOT NULL DEFAULT now(), ADD COLUMN solo_since timestamptz;

CREATE TABLE meet_voice_participants (
    id uuid PRIMARY KEY,
    user_id uuid REFERENCES profiles(id) ON DELETE SET NULL,
    post_id uuid NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    room_sid text NOT NULL REFERENCES meet_voice_sessions(room_sid) ON DELETE CASCADE,
    public_name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    ended_at timestamptz,
    revoked_at timestamptz
);
CREATE INDEX meet_voice_participants_usage_idx ON meet_voice_participants(created_at, ended_at);
ALTER TABLE meet_voice_participants ENABLE ROW LEVEL SECURITY;
CREATE TABLE meet_voice_actions (
    id uuid PRIMARY KEY,
    actor_id uuid REFERENCES profiles(id) ON DELETE SET NULL,
    connection_id uuid,
    target_user_id uuid REFERENCES profiles(id) ON DELETE SET NULL,
    post_id uuid REFERENCES posts(id) ON DELETE SET NULL,
    action text NOT NULL CHECK(action IN ('mute','remove','ban','unban','pause','resume','configure','timeout','quota')),
    success boolean NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE meet_voice_actions ENABLE ROW LEVEL SECURITY;
ALTER TABLE reports DROP CONSTRAINT reports_target_type_check;
ALTER TABLE reports ADD CONSTRAINT reports_target_type_check CHECK (target_type IN ('post','comment','injury_note','review_response','coaching_message','practice_rating','coaching_consultation','voice_participant'));
