ALTER TABLE review_responses ADD COLUMN clarify_body text CHECK (char_length(clarify_body) <= 2000);

-- 元の依頼・契約が退会で消えても、会話と会計の記録を保持するため親IDには外部キーを張らない。
CREATE TABLE coaching_consultations (
    id uuid PRIMARY KEY,
    requester_id uuid REFERENCES profiles(id) ON DELETE SET NULL,
    coach_id uuid REFERENCES profiles(id) ON DELETE SET NULL,
    request_key uuid NOT NULL,
    review_request_id uuid,
    contract_id uuid,
    minor boolean NOT NULL,
    funding text NOT NULL CHECK (funding IN ('ticket','cash')),
    price_yen integer NOT NULL CHECK (price_yen BETWEEN 500 AND 9999999),
    share_percent smallint NOT NULL CHECK (share_percent BETWEEN 1 AND 100),
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','awaiting_payment','offered','in_review','answered','declined','expired','cancelled')),
    question text NOT NULL DEFAULT '' CHECK (char_length(question) <= 2000),
    video_public_id text UNIQUE,
    answer text NOT NULL DEFAULT '' CHECK (char_length(answer) <= 2000),
    answer_video_public_id text UNIQUE,
    clarify_body text CHECK (char_length(clarify_body) BETWEEN 1 AND 2000),
    clarification text CHECK (char_length(clarification) BETWEEN 1 AND 2000),
    clarify_requested_at timestamptz,
    clarified_at timestamptz,
    respond_by timestamptz,
    answer_by timestamptz,
    answered_at timestamptz,
    closed_at timestamptz,
    expires_at timestamptz NOT NULL DEFAULT now()+interval '7 days',
    payment_state text NOT NULL DEFAULT 'unpaid' CHECK (payment_state IN ('unpaid','paid','refund_pending','refunded')),
    checkout_session_id text UNIQUE,
    checkout_url text,
    session_expires_at timestamptz,
    payment_intent_id text UNIQUE,
    paid_at timestamptz,
    hidden_at timestamptz,
    purged_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(requester_id,request_key),
    CHECK (num_nonnulls(review_request_id,contract_id)=1),
    CHECK ((review_request_id IS NOT NULL AND funding='ticket') OR (contract_id IS NOT NULL AND funding='cash')),
    CHECK (requester_id IS DISTINCT FROM coach_id OR requester_id IS NULL),
    CHECK (payment_state='unpaid' OR (payment_intent_id IS NOT NULL AND paid_at IS NOT NULL)),
    CHECK (status <> 'answered' OR answered_at IS NOT NULL)
);
CREATE UNIQUE INDEX coaching_consultations_open_review_idx ON coaching_consultations(review_request_id)
    WHERE status IN ('draft','awaiting_payment','offered','in_review');
CREATE UNIQUE INDEX coaching_consultations_open_contract_idx ON coaching_consultations(contract_id)
    WHERE status IN ('draft','awaiting_payment','offered','in_review');
CREATE INDEX coaching_consultations_coach_idx ON coaching_consultations(coach_id,status,created_at);
CREATE INDEX coaching_consultations_contract_idx ON coaching_consultations(contract_id,created_at);
CREATE INDEX coaching_consultations_review_idx ON coaching_consultations(review_request_id,created_at);

ALTER TABLE coaching_ticket_reservations ADD COLUMN consultation_id uuid UNIQUE;
ALTER TABLE coach_earnings ADD COLUMN consultation_id uuid UNIQUE;
ALTER TABLE coach_earnings DROP CONSTRAINT coach_earnings_source_check;
ALTER TABLE coach_earnings ADD CONSTRAINT coach_earnings_source_check CHECK (source IN ('review','personal','consultation'));

CREATE TABLE coaching_consultation_refunds (
    id uuid PRIMARY KEY,
    consultation_id uuid NOT NULL REFERENCES coaching_consultations(id),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','succeeded','failed')),
    stripe_refund_id text UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX coaching_consultation_refunds_pending_idx ON coaching_consultation_refunds(consultation_id) WHERE status='pending';

ALTER TABLE reports DROP CONSTRAINT reports_target_type_check;
ALTER TABLE reports ADD CONSTRAINT reports_target_type_check CHECK (target_type IN ('post','comment','injury_note','review_response','coaching_message','practice_rating','coaching_consultation'));
ALTER TABLE reports DROP CONSTRAINT reports_by_guardian_check;
ALTER TABLE reports ADD CONSTRAINT reports_by_guardian_check CHECK (NOT by_guardian OR target_type IN ('coaching_message','coaching_consultation'));
CREATE TABLE coaching_consultation_access_logs (
    id uuid PRIMARY KEY,
    admin_id uuid NOT NULL,
    consultation_id uuid NOT NULL,
    report_id uuid NOT NULL,
    opened_at timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER coaching_consultation_access_logs_immutable BEFORE UPDATE OR DELETE ON coaching_consultation_access_logs
    FOR EACH ROW EXECUTE FUNCTION coaching_access_logs_immutable();
ALTER TABLE coaching_consultations ENABLE ROW LEVEL SECURITY;
ALTER TABLE coaching_consultation_refunds ENABLE ROW LEVEL SECURITY;
ALTER TABLE coaching_consultation_access_logs ENABLE ROW LEVEL SECURITY;
