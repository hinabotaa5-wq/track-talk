-- 回数制限の打ち切りを permanent ではなく rate_limited として残せるようにする。
-- 選定上は transient/permanent と同じ後回しで、サマリの表記だけ区別する。
ALTER TABLE competition_import_attempts DROP CONSTRAINT competition_import_attempts_outcome_chk;
ALTER TABLE competition_import_attempts ADD CONSTRAINT competition_import_attempts_outcome_chk
    CHECK (outcome IN ('success', 'partial', 'transient', 'permanent', 'skipped', 'rate_limited'));
