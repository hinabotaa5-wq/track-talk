-- お試しプランは単発に戻す（値段と日数は単発の決まりにそのまま収まる）。
UPDATE coaching_contracts SET plan_kind = 'single' WHERE plan_kind = 'trial';
UPDATE coaching_plans SET kind = 'single' WHERE kind = 'trial';

ALTER TABLE coaching_contracts DROP CONSTRAINT coaching_contracts_plan_kind_check;
ALTER TABLE coaching_contracts
    ADD CONSTRAINT coaching_contracts_plan_kind_check CHECK (plan_kind IN ('period', 'single'));

ALTER TABLE coaching_plans DROP CONSTRAINT coaching_plans_trial_price_check;
ALTER TABLE coaching_plans DROP CONSTRAINT coaching_plans_check;
ALTER TABLE coaching_plans DROP CONSTRAINT coaching_plans_kind_check;
ALTER TABLE coaching_plans
    ADD CONSTRAINT coaching_plans_kind_check CHECK (kind IN ('period', 'single')),
    ADD CONSTRAINT coaching_plans_check CHECK ((kind = 'period' AND duration_days IN (14, 30)) OR (kind = 'single' AND duration_days = 7));
