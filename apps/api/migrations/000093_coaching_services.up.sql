-- 既存プランの内容は推測せず、分類は未設定のまま残す。
ALTER TABLE coach_profiles
    ADD COLUMN service_types text[] NOT NULL DEFAULT '{}',
    ADD CONSTRAINT coach_profiles_service_types_check CHECK (
        cardinality(service_types) <= 3
        AND array_position(service_types, NULL) IS NULL
        AND service_types <@ ARRAY['video_review', 'competition_support', 'in_person']::text[]
    );

ALTER TABLE coaching_plans
    ADD COLUMN service_type text CHECK (service_type IN ('video_review', 'competition_support', 'in_person')),
    ADD COLUMN additional_consultation_price_yen integer CHECK (additional_consultation_price_yen BETWEEN 500 AND 9999999);

-- 購入時点の内容。以前の契約は内容を復元できないので NULL とする。
ALTER TABLE coaching_contracts
    ADD COLUMN service_type text CHECK (service_type IN ('video_review', 'competition_support', 'in_person')),
    ADD COLUMN plan_description text,
    ADD COLUMN additional_consultation_price_yen integer CHECK (additional_consultation_price_yen BETWEEN 500 AND 9999999);
