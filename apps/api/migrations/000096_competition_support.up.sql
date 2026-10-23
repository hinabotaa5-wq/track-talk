ALTER TABLE coaching_plans
 ADD COLUMN competition_name text CHECK (char_length(competition_name) BETWEEN 1 AND 100),
 ADD COLUMN competition_date date,
 ADD COLUMN support_start_date date,
 ADD COLUMN call_count smallint NOT NULL DEFAULT 0 CHECK (call_count BETWEEN 0 AND 12),
 ADD COLUMN call_minutes smallint NOT NULL DEFAULT 30 CHECK (call_minutes IN (15,30,45,60));
ALTER TABLE coaching_plans DROP CONSTRAINT coaching_plans_check;
ALTER TABLE coaching_plans ADD CONSTRAINT coaching_plans_check CHECK (
 (service_type='competition_support' AND kind='period' AND duration_days BETWEEN 1 AND 90)
 OR (kind='period' AND duration_days IN (14,30)) OR (kind IN ('single','trial') AND duration_days=7));
ALTER TABLE coaching_plans ADD CONSTRAINT support_plan_dates_check CHECK
 (competition_date IS NULL OR support_start_date IS NULL OR competition_date>=support_start_date);
ALTER TABLE coaching_contracts
 ADD COLUMN competition_name text,
 ADD COLUMN competition_date date,
 ADD COLUMN support_start_date date,
 ADD COLUMN call_count smallint NOT NULL DEFAULT 0,
 ADD COLUMN call_minutes smallint NOT NULL DEFAULT 30,
 ADD COLUMN fulfillment_version smallint NOT NULL DEFAULT 0 CHECK (fulfillment_version IN (0,1)),
 ADD COLUMN fulfillment_state text NOT NULL DEFAULT 'open' CHECK (fulfillment_state IN ('open','refund_pending','refund_failed','settled')),
 ADD COLUMN support_refund_amount_yen integer,
 ADD COLUMN support_refund_attempt_id uuid,
 ADD COLUMN support_refund_reason text;
ALTER TABLE coaching_messages ADD COLUMN review_submission_id uuid;
CREATE TABLE coaching_video_reviews (
 question_id uuid PRIMARY KEY REFERENCES coaching_messages(id) ON DELETE CASCADE,
 contract_id uuid NOT NULL REFERENCES coaching_contracts(id) ON DELETE CASCADE,
 answer_id uuid UNIQUE REFERENCES coaching_messages(id) ON DELETE SET NULL,
 answered_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX coaching_video_reviews_contract_idx ON coaching_video_reviews(contract_id);
CREATE TABLE coaching_calls (
 id uuid PRIMARY KEY,
 contract_id uuid NOT NULL REFERENCES coaching_contracts(id) ON DELETE CASCADE,
 coach_id uuid NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
 requester_id uuid NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
 request_key uuid NOT NULL,
 starts_at timestamptz NOT NULL,
 ends_at timestamptz NOT NULL CHECK (ends_at>starts_at),
 join_url text NOT NULL,
 status text NOT NULL DEFAULT 'proposed' CHECK (status IN ('proposed','confirmed','completion_pending','completed','cancelled')),
 reported_at timestamptz,
 completed_at timestamptz,
 cancelled_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(contract_id,request_key)
);
CREATE INDEX coaching_calls_coach_idx ON coaching_calls(coach_id,starts_at) WHERE status IN ('proposed','confirmed','completion_pending');
CREATE INDEX coaching_calls_requester_idx ON coaching_calls(requester_id,starts_at) WHERE status IN ('proposed','confirmed','completion_pending');
ALTER TABLE coaching_video_reviews ENABLE ROW LEVEL SECURITY;
ALTER TABLE coaching_calls ENABLE ROW LEVEL SECURITY;
