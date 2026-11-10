-- 000152 の巻き戻し。残っている rate_limited は transient に倒してから制約を戻す。
UPDATE competition_import_attempts SET outcome = 'transient' WHERE outcome = 'rate_limited';
ALTER TABLE competition_import_attempts DROP CONSTRAINT competition_import_attempts_outcome_chk;
ALTER TABLE competition_import_attempts ADD CONSTRAINT competition_import_attempts_outcome_chk
    CHECK (outcome IN ('success', 'partial', 'transient', 'permanent', 'skipped'));
