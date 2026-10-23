ALTER TABLE coaching_plans
 ADD COLUMN venue_area text CHECK (char_length(venue_area) BETWEEN 1 AND 100),
 ADD COLUMN venue_name text CHECK (char_length(venue_name) BETWEEN 1 AND 100),
 ADD COLUMN session_count smallint NOT NULL DEFAULT 0 CHECK (session_count BETWEEN 0 AND 12),
 ADD COLUMN session_minutes smallint NOT NULL DEFAULT 60 CHECK (session_minutes IN (30,60,90,120)),
 ADD COLUMN travel_fee_yen integer NOT NULL DEFAULT 0 CHECK (travel_fee_yen BETWEEN 0 AND 100000),
 ADD COLUMN max_participants smallint NOT NULL DEFAULT 1 CHECK (max_participants BETWEEN 1 AND 20);
ALTER TABLE coaching_contracts
 ADD COLUMN venue_area text,
 ADD COLUMN venue_name text,
 ADD COLUMN session_count smallint NOT NULL DEFAULT 0,
 ADD COLUMN session_minutes smallint NOT NULL DEFAULT 60,
 ADD COLUMN travel_fee_yen integer NOT NULL DEFAULT 0,
 ADD COLUMN max_participants smallint NOT NULL DEFAULT 1,
 ADD COLUMN delivered_session_count integer NOT NULL DEFAULT 0;
ALTER TABLE coaching_contracts DROP CONSTRAINT coaching_contracts_fulfillment_version_check;
ALTER TABLE coaching_contracts ADD CONSTRAINT coaching_contracts_fulfillment_version_check CHECK (fulfillment_version IN (0,1,2));
CREATE TABLE coaching_in_person_sessions (
 id uuid PRIMARY KEY,
 contract_id uuid NOT NULL REFERENCES coaching_contracts(id) ON DELETE CASCADE,
 coach_id uuid NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
 requester_id uuid NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
 request_key uuid NOT NULL,
 starts_at timestamptz NOT NULL,
 ends_at timestamptz NOT NULL CHECK (ends_at>starts_at),
 venue_name text NOT NULL,
 venue_area text NOT NULL,
 meeting_note text NOT NULL DEFAULT '',
 participants smallint NOT NULL DEFAULT 1 CHECK (participants BETWEEN 1 AND 20),
 status text NOT NULL DEFAULT 'proposed' CHECK (status IN ('proposed','confirmed','completion_pending','completed','cancelled')),
 reported_at timestamptz,
 completed_at timestamptz,
 cancelled_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(contract_id,request_key)
);
CREATE INDEX coaching_in_person_coach_idx ON coaching_in_person_sessions(coach_id,starts_at) WHERE status IN ('proposed','confirmed','completion_pending');
CREATE INDEX coaching_in_person_requester_idx ON coaching_in_person_sessions(requester_id,starts_at) WHERE status IN ('proposed','confirmed','completion_pending');
ALTER TABLE coaching_in_person_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE coaching_payments DROP CONSTRAINT coaching_payments_refund_reason_check;
ALTER TABLE coaching_payments ADD CONSTRAINT coaching_payments_refund_reason_check CHECK (refund_reason IN ('no_coach_reply', 'not_started', 'admin_full', 'admin_remaining', 'support_unprovided', 'support_admin_full', 'support_no_response', 'in_person_unprovided', 'in_person_admin_full'));
