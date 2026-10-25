-- 接続の正本はLiveKit。予約は参加券の二重発行と別ルームへの同時参加を防ぐ。
CREATE TABLE meet_voice_reservations (
    user_id uuid PRIMARY KEY REFERENCES profiles(id) ON DELETE CASCADE,
    id uuid NOT NULL UNIQUE,
    post_id uuid NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    token_expires_at timestamptz NOT NULL,
    state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'connected', 'disconnected')),
    participant_sid text NOT NULL DEFAULT '',
    last_event_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX meet_voice_reservations_post_idx ON meet_voice_reservations(post_id);
ALTER TABLE meet_voice_reservations ENABLE ROW LEVEL SECURITY;

CREATE TABLE meet_voice_sessions (
    room_sid text PRIMARY KEY,
    post_id uuid NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    room_name text NOT NULL,
    started_at timestamptz NOT NULL,
    ended_at timestamptz,
    last_event_at timestamptz NOT NULL
);
CREATE INDEX meet_voice_sessions_post_idx ON meet_voice_sessions(post_id, started_at DESC);
ALTER TABLE meet_voice_sessions ENABLE ROW LEVEL SECURITY;

CREATE TABLE meet_voice_bans (
    post_id uuid NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    reason text NOT NULL DEFAULT '',
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (post_id, user_id)
);
ALTER TABLE meet_voice_bans ENABLE ROW LEVEL SECURITY;

CREATE TABLE meet_voice_rate_limits (
    user_id uuid PRIMARY KEY REFERENCES profiles(id) ON DELETE CASCADE,
    window_start timestamptz NOT NULL,
    attempts integer NOT NULL CHECK (attempts BETWEEN 1 AND 6)
);
ALTER TABLE meet_voice_rate_limits ENABLE ROW LEVEL SECURITY;

-- 生のWebhook・参加券・音声・プロフィールは保存しない。
CREATE TABLE meet_voice_webhook_events (
    id uuid PRIMARY KEY,
    event text NOT NULL,
    room_sid text NOT NULL,
    created_at timestamptz NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE meet_voice_webhook_events ENABLE ROW LEVEL SECURITY;
CREATE INDEX meet_voice_webhook_events_received_idx ON meet_voice_webhook_events(received_at);
