DROP TABLE coach_registration_endings;
-- 登録を終えたコーチは、本人がやめた扱いに寄せてから元の制約に戻す（どちらも承認済みでないので送金の扱いは変わらない）。
UPDATE coach_profiles SET status = 'withdrawn' WHERE status = 'ended';
ALTER TABLE coach_profiles DROP CONSTRAINT coach_profiles_status_check;
ALTER TABLE coach_profiles ADD CONSTRAINT coach_profiles_status_check
    CHECK (status IN ('pending', 'approved', 'rejected', 'suspended', 'withdrawn'));
