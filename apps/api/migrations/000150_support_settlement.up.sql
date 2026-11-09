-- 大会までプラン（無料相談の提案から成立した、動画への返答が無制限の契約）の途中終了と、コーチ報酬の精算。
-- 解約料は取らない。未経過分は期間の日割りで全額返金する。コーチ報酬は契約終了時に経過日数の日割りで確定し、
-- 月次送金はそれまでの「確定前の仮払い」とする。コーチ都合（帰責事由あり）は選手へ全額返金し、
-- コーチは返金額のコーチ報酬相当分（80%）を上限に仮払いを返す（没収ではなく返還・精算）。

-- 終わり方と、日割りの終わりの日。cause は、player（選手の都合）、coach_fault（コーチ都合・帰責事由あり）、
-- unavoidable（やむを得ない事情）、competition_cancelled（大会の中止・延期。主催者の正式発表の日で終わる）。
-- coach_hold は、コーチ都合で返金と精算が終わるまで、その契約のコーチ分の仮払いを止めている印。
ALTER TABLE coaching_contracts
    ADD COLUMN support_end_cause text CHECK (support_end_cause IN ('player', 'coach_fault', 'unavoidable', 'competition_cancelled')),
    ADD COLUMN support_end_on date,
    ADD COLUMN support_coach_hold boolean NOT NULL DEFAULT false,
    ADD CONSTRAINT coaching_contracts_support_end_check CHECK ((support_end_cause IS NULL) = (support_end_on IS NULL));

-- 返金の理由に、大会までプランの途中終了の4つを足す。
ALTER TABLE coaching_payments DROP CONSTRAINT coaching_payments_refund_reason_check;
ALTER TABLE coaching_payments ADD CONSTRAINT coaching_payments_refund_reason_check CHECK (refund_reason IN ('no_coach_reply', 'not_started', 'admin_full', 'admin_remaining', 'support_unprovided', 'support_admin_full', 'support_no_response', 'in_person_unprovided', 'in_person_admin_full', 'support_player', 'support_coach_fault', 'support_unavoidable', 'support_competition_cancelled'));

ALTER TABLE coach_earnings DROP CONSTRAINT coach_earnings_source_check;
ALTER TABLE coach_earnings ADD CONSTRAINT coach_earnings_source_check
    CHECK (source IN ('review', 'personal', 'consultation', 'refund_compensation', 'support_advance'));

-- 月次の仮払い。その月の終わりまでの経過日数で計算した報酬の累計から、前回までの仮払いを引いた額。
-- 送金前の仮払いの報酬が消えた（コーチ都合で止めた）ときは、この行も消える。
CREATE TABLE support_coach_advances (
    earning_id uuid PRIMARY KEY REFERENCES coach_earnings (id) ON DELETE CASCADE,
    contract_id uuid NOT NULL REFERENCES coaching_contracts (id),
    through_date date NOT NULL,
    elapsed_days integer NOT NULL CHECK (elapsed_days > 0),
    total_days integer NOT NULL CHECK (total_days >= elapsed_days),
    amount_yen integer NOT NULL CHECK (amount_yen > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (contract_id, through_date)
);
CREATE INDEX support_coach_advances_contract_idx ON support_coach_advances (contract_id);

-- 契約終了時の確定。final_yen は確定した報酬から仮払いを引いて新しく記録した額（0 もある）、
-- clawback_yen はコーチ都合で仮払いから返してもらう額（返金額の80%が上限）。
CREATE TABLE support_coach_settlements (
    contract_id uuid PRIMARY KEY REFERENCES coaching_contracts (id),
    earned_yen integer NOT NULL CHECK (earned_yen >= 0),
    advances_yen integer NOT NULL CHECK (advances_yen >= 0),
    final_yen integer NOT NULL CHECK (final_yen >= 0),
    clawback_yen integer NOT NULL DEFAULT 0 CHECK (clawback_yen >= 0),
    created_at timestamptz NOT NULL DEFAULT now()
);

-- コーチ都合かやむを得ない事情かの判定。理由は書面（電磁的記録）としてコーチに通知し、異議申立て（再審査）は1回。
CREATE TABLE support_fault_determinations (
    contract_id uuid PRIMARY KEY REFERENCES coaching_contracts (id),
    decision text NOT NULL CHECK (decision IN ('coach_fault', 'unavoidable')),
    reason text NOT NULL CHECK (char_length(btrim(reason)) BETWEEN 1 AND 2000),
    decided_by uuid REFERENCES profiles (id) ON DELETE SET NULL,
    notified_at timestamptz NOT NULL DEFAULT now(),
    objection text CHECK (objection IS NULL OR char_length(btrim(objection)) BETWEEN 1 AND 2000),
    objected_at timestamptz,
    review_decision text CHECK (review_decision IN ('coach_fault', 'unavoidable')),
    review_reason text CHECK (review_reason IS NULL OR char_length(btrim(review_reason)) BETWEEN 1 AND 2000),
    reviewed_by uuid REFERENCES profiles (id) ON DELETE SET NULL,
    reviewed_at timestamptz,
    CHECK ((objection IS NULL) = (objected_at IS NULL)),
    CHECK ((review_decision IS NULL) = (reviewed_at IS NULL) AND (review_decision IS NULL) = (review_reason IS NULL)),
    CHECK (reviewed_at IS NULL OR objected_at IS NOT NULL)
);

-- コーチ都合の仮払いの返還は、既存の返金相殺の台帳（翌月以降の報酬から差し引く）に載せる。
ALTER TABLE coach_refund_adjustments ALTER COLUMN refund_id DROP NOT NULL;
ALTER TABLE coach_refund_adjustments ADD COLUMN contract_id uuid REFERENCES coaching_contracts (id);
ALTER TABLE coach_refund_adjustments DROP CONSTRAINT coach_refund_adjustments_kind_check;
ALTER TABLE coach_refund_adjustments ADD CONSTRAINT coach_refund_adjustments_kind_check
    CHECK (kind IN ('refund_offset', 'compensation_recovery', 'support_clawback'));
ALTER TABLE coach_refund_adjustments DROP CONSTRAINT coach_refund_adjustments_earning_required;
ALTER TABLE coach_refund_adjustments ADD CONSTRAINT coach_refund_adjustments_earning_required
    CHECK (kind IN ('compensation_recovery', 'support_clawback') OR earning_id IS NOT NULL);
ALTER TABLE coach_refund_adjustments ADD CONSTRAINT coach_refund_adjustments_source_check
    CHECK (CASE WHEN kind = 'support_clawback' THEN contract_id IS NOT NULL AND refund_id IS NULL ELSE refund_id IS NOT NULL END);
CREATE UNIQUE INDEX coach_refund_adjustments_support_idx ON coach_refund_adjustments (contract_id) WHERE kind = 'support_clawback';

-- コーチへの取引条件の明示（フリーランス法第3条）。個別の契約ごとに、成立の時点で電磁的に記録して示す。
CREATE TABLE support_contract_statements (
    contract_id uuid PRIMARY KEY REFERENCES coaching_contracts (id) ON DELETE CASCADE,
    coach_id uuid REFERENCES profiles (id) ON DELETE SET NULL,
    body jsonb NOT NULL,
    issued_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE support_coach_advances ENABLE ROW LEVEL SECURITY;
ALTER TABLE support_coach_settlements ENABLE ROW LEVEL SECURITY;
ALTER TABLE support_fault_determinations ENABLE ROW LEVEL SECURITY;
ALTER TABLE support_contract_statements ENABLE ROW LEVEL SECURITY;
