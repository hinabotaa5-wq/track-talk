-- やめたコーチは、運営が止めた扱いに寄せてから元の制約に戻す。
UPDATE coach_profiles SET status = 'suspended' WHERE status = 'withdrawn';
ALTER TABLE coach_profiles DROP CONSTRAINT coach_profiles_status_check;
ALTER TABLE coach_profiles ADD CONSTRAINT coach_profiles_status_check
    CHECK (status IN ('pending', 'approved', 'rejected', 'suspended'));
