-- 県ごとの大会取り込みの試行の記録。失敗しても残るので、翌日の選定で失敗県を後回しにして後続の県に予算を届けられる。
CREATE TABLE IF NOT EXISTS competition_import_attempts (
    prefecture text PRIMARY KEY,
    attempted_at timestamptz NOT NULL DEFAULT now(),
    outcome text NOT NULL
        CONSTRAINT competition_import_attempts_outcome_chk
        CHECK (outcome IN ('success', 'partial', 'transient', 'permanent', 'skipped')),
    consecutive_failures integer NOT NULL DEFAULT 0,
    failed_urls text[] NOT NULL DEFAULT '{}',
    requests_made integer NOT NULL DEFAULT 0,
    CONSTRAINT competition_import_attempts_prefecture_chk CHECK (prefecture ~ '^(0[1-9]|[1-3][0-9]|4[0-7])$')
);

-- 000073 の competition_news_checks と同じく RLS だけ有効にする。
ALTER TABLE competition_import_attempts ENABLE ROW LEVEL SECURITY;
