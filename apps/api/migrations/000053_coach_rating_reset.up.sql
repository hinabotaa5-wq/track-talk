-- 評価が低くて打診の順番から外れたコーチを、運営が戻せるようにする。
-- 戻した時刻より前の★は、外す判定に数えない。
ALTER TABLE coach_profiles
    ADD COLUMN rating_reset_at timestamptz;
