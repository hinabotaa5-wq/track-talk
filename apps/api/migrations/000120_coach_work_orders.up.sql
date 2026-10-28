-- フリーランス保護法3条の取引条件の明示。運営者が販売するフォーム添削と、その追加相談を
-- コーチが引き受けるたびに、その案件の条件(9項目)を書き留める。後から書き換えない。
-- サポートプラン(コーチが販売者)の分は、弁護士の回答(Q9)を待ってから決める。
CREATE TABLE coach_work_orders (
    id uuid PRIMARY KEY,
    kind text NOT NULL CHECK (kind IN ('review', 'consultation')),
    review_request_id uuid UNIQUE REFERENCES review_requests (id) ON DELETE SET NULL,
    consultation_id uuid UNIQUE REFERENCES coaching_consultations (id) ON DELETE SET NULL,
    coach_id uuid REFERENCES profiles (id) ON DELETE SET NULL,
    terms_version smallint NOT NULL,
    terms jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((kind = 'review') = (consultation_id IS NULL))
);
CREATE INDEX coach_work_orders_coach_idx ON coach_work_orders (coach_id, created_at DESC);
ALTER TABLE coach_work_orders ENABLE ROW LEVEL SECURITY;
