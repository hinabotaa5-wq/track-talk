-- パーソナル指導の保護者の参加と、通報・運営の閲覧。

-- 新しい状態。
-- accepted: コーチが承諾し、18歳未満の利用者の保護者の同意を待っている（決済を入れたら、支払い待ちにも使う）。
-- guardian_declined: 保護者が同意しなかった。suspended: 通報などで運営が止めた。
ALTER TABLE coaching_contracts
    ADD COLUMN start_by timestamptz,
    ADD COLUMN suspended_at timestamptz,
    DROP CONSTRAINT coaching_contracts_status_check,
    ADD CONSTRAINT coaching_contracts_status_check CHECK (status IN (
        'applied', 'accepted', 'active', 'suspended', 'ended', 'declined', 'guardian_declined', 'expired', 'cancelled'
    )),
    DROP CONSTRAINT coaching_contracts_check1,
    ADD CONSTRAINT coaching_contracts_period_check CHECK (
        status NOT IN ('active', 'suspended', 'ended') OR (starts_at IS NOT NULL AND ends_at IS NOT NULL)
    ),
    ADD CONSTRAINT coaching_contracts_start_by_check CHECK (status <> 'accepted' OR start_by IS NOT NULL),
    ADD CONSTRAINT coaching_contracts_suspended_check CHECK (status <> 'suspended' OR suspended_at IS NOT NULL);

DROP INDEX coaching_contracts_open_idx;
CREATE UNIQUE INDEX coaching_contracts_open_idx ON coaching_contracts (requester_id, coach_id)
    WHERE status IN ('applied', 'accepted', 'active', 'suspended');
DROP INDEX coaching_contracts_plan_idx;
CREATE INDEX coaching_contracts_plan_idx ON coaching_contracts (plan_id)
    WHERE status IN ('applied', 'accepted', 'active', 'suspended');

-- 保護者も契約ごとに同じ2つに同意する。保護者はアカウントを持たないので profile_id は空。
ALTER TABLE coaching_contract_consents
    ALTER COLUMN profile_id DROP NOT NULL,
    DROP CONSTRAINT coaching_contract_consents_party_check,
    ADD CONSTRAINT coaching_contract_consents_party_check CHECK (party IN ('requester', 'coach', 'guardian')),
    ADD CONSTRAINT coaching_contract_consents_profile_check CHECK ((party = 'guardian') = (profile_id IS NULL));

-- やりとり欄の1通を通報できるようにする。保護者はアカウントを持たないので、子のアカウントで by_guardian を立てる。
ALTER TABLE reports
    ADD COLUMN by_guardian boolean NOT NULL DEFAULT false,
    DROP CONSTRAINT reports_target_type_check,
    ADD CONSTRAINT reports_target_type_check CHECK (target_type IN ('post', 'comment', 'injury_note', 'review_response', 'coaching_message')),
    ADD CONSTRAINT reports_by_guardian_check CHECK (NOT by_guardian OR target_type = 'coaching_message'),
    DROP CONSTRAINT reports_reporter_id_target_type_target_id_key,
    ADD CONSTRAINT reports_reporter_target_key UNIQUE (reporter_id, by_guardian, target_type, target_id);

-- 運営がやりとり欄を開いた記録。誰が、どの契約を、どの通報で、いつ。
-- 退会などで契約が消えても記録は残すので、外部キーは張らない。更新と削除はトリガーで止める。
CREATE TABLE coaching_access_logs (
    id uuid PRIMARY KEY,
    admin_id uuid NOT NULL,
    contract_id uuid NOT NULL,
    report_id uuid NOT NULL,
    opened_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX coaching_access_logs_contract_idx ON coaching_access_logs (contract_id, opened_at DESC);

ALTER TABLE coaching_access_logs ENABLE ROW LEVEL SECURITY;

CREATE FUNCTION coaching_access_logs_immutable() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'coaching_access_logs は変更・削除できません';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER coaching_access_logs_immutable
    BEFORE UPDATE OR DELETE ON coaching_access_logs
    FOR EACH ROW EXECUTE FUNCTION coaching_access_logs_immutable();
