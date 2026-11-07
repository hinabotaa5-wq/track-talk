CREATE TABLE coach_stripe_account_attempts (
    profile_id uuid PRIMARY KEY REFERENCES coach_profiles(profile_id) ON DELETE CASCADE,
    idempotency_key text NOT NULL,
    started_at timestamptz
);
ALTER TABLE coach_stripe_account_attempts ENABLE ROW LEVEL SECURITY;
