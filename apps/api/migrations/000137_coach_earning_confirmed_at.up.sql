ALTER TABLE coach_earnings ADD COLUMN confirmed_at timestamptz;
UPDATE coach_earnings SET confirmed_at = created_at;
ALTER TABLE coach_earnings ALTER COLUMN confirmed_at SET NOT NULL;

CREATE INDEX coach_earnings_coach_confirmed_idx ON coach_earnings (coach_id, confirmed_at DESC);
CREATE INDEX coach_earnings_unpaid_confirmed_idx ON coach_earnings (confirmed_at) WHERE payout_id IS NULL;
