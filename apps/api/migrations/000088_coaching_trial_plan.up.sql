-- お試しプラン。同じコーチの指導を受けたことがない人だけが申し込める、7日で終わる安い単発。
-- 値段は2,000〜3,000円。1人のコーチが持てるお試しプランは1つまで（API で確かめる）。
ALTER TABLE coaching_plans DROP CONSTRAINT coaching_plans_kind_check;
ALTER TABLE coaching_plans DROP CONSTRAINT coaching_plans_check;
ALTER TABLE coaching_plans
    ADD CONSTRAINT coaching_plans_kind_check CHECK (kind IN ('period', 'single', 'trial')),
    ADD CONSTRAINT coaching_plans_check CHECK (
        (kind = 'period' AND duration_days IN (14, 30))
        OR (kind IN ('single', 'trial') AND duration_days = 7)
    ),
    ADD CONSTRAINT coaching_plans_trial_price_check CHECK (kind <> 'trial' OR price_yen <= 3000);

ALTER TABLE coaching_contracts DROP CONSTRAINT coaching_contracts_plan_kind_check;
ALTER TABLE coaching_contracts
    ADD CONSTRAINT coaching_contracts_plan_kind_check CHECK (plan_kind IN ('period', 'single', 'trial'));
