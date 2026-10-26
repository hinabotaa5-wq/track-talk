-- 外部AIの呼び出し回数だけを記録。本文・利用者ID・応答は保存しない。
CREATE TABLE sense_summary_attempts (
    id bigserial PRIMARY KEY,
    attempted_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX sense_summary_attempts_attempted_at_idx ON sense_summary_attempts (attempted_at);
ALTER TABLE sense_summary_attempts ENABLE ROW LEVEL SECURITY;
-- 更新前の直近24時間の成功分も数え、配備で枠をリセットしない。
INSERT INTO sense_summary_attempts (attempted_at)
SELECT created_at FROM sense_note_summaries WHERE created_at > now() - interval '24 hours';
