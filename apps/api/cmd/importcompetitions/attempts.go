package main

import (
	"context"
	"log"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/competitions"
	"github.com/jackc/pgx/v5/pgxpool"
)

// 試行の記録は competition_import_attempts に置く。失敗しても残るので、
// 翌日の DueSourcesWithAttempts が失敗県を後回しにして後続の県に予算を届けられる。
// テーブルがまだない本番（migration 未適用）でも取り込み自体は止めない。
// 読み書きはどちらも失敗をログに残すだけで、取り込みの成否には影響させない。

const ensureImportAttemptsDDL = `CREATE TABLE IF NOT EXISTS competition_import_attempts (
    prefecture text PRIMARY KEY,
    attempted_at timestamptz NOT NULL DEFAULT now(),
    outcome text NOT NULL
        CONSTRAINT competition_import_attempts_outcome_chk
        CHECK (outcome IN ('success', 'partial', 'transient', 'permanent', 'skipped', 'rate_limited')),
    consecutive_failures integer NOT NULL DEFAULT 0,
    failed_urls text[] NOT NULL DEFAULT '{}',
    requests_made integer NOT NULL DEFAULT 0,
    CONSTRAINT competition_import_attempts_prefecture_chk CHECK (prefecture ~ '^(0[1-9]|[1-3][0-9]|4[0-7])$')
)`

func ensureImportAttemptsTable(ctx context.Context, pool *pgxpool.Pool) {
	if _, err := pool.Exec(ctx, ensureImportAttemptsDDL); err != nil {
		log.Printf("import attempts table: %v", err)
		return
	}
	// ほかの取り込み表と同じく RLS だけ有効にする（方針は competition_news_checks と同じ）。
	// 失敗しても取り込みは続ける。
	if _, err := pool.Exec(ctx, `ALTER TABLE competition_import_attempts ENABLE ROW LEVEL SECURITY`); err != nil {
		log.Printf("import attempts table: %v", err)
	}
}

func loadImportAttempts(ctx context.Context, pool *pgxpool.Pool) map[string]competitions.Attempt {
	rows, err := pool.Query(ctx, `SELECT prefecture, attempted_at, outcome, consecutive_failures, failed_urls, requests_made FROM competition_import_attempts`)
	if err != nil {
		log.Printf("import attempts: %v", err)
		return nil
	}
	defer rows.Close()
	attempts := map[string]competitions.Attempt{}
	for rows.Next() {
		var prefecture, outcome string
		var at time.Time
		var fails, made int
		var failed []string
		if err := rows.Scan(&prefecture, &at, &outcome, &fails, &failed, &made); err != nil {
			log.Printf("import attempts: %v", err)
			return nil
		}
		attempts[prefecture] = competitions.Attempt{
			Outcome:             outcome,
			AttemptedAt:         at,
			ConsecutiveFailures: fails,
			FailedURLs:          failed,
			RequestsMade:        made,
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("import attempts: %v", err)
		return nil
	}
	return attempts
}

func recordImportAttempt(ctx context.Context, pool *pgxpool.Pool, prefecture string, attempt competitions.Attempt) {
	if attempt.FailedURLs == nil {
		attempt.FailedURLs = []string{}
	}
	// 試行の記録は、県あたりの期限切れ・全体の期限切れのときも残す。
	// 親の期限が切れていると Exec まで道連れになるので、書き込みだけ切り離す。
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	_, err := pool.Exec(wctx, `INSERT INTO competition_import_attempts
		(prefecture, attempted_at, outcome, consecutive_failures, failed_urls, requests_made)
		VALUES ($1, now(), $2, $3, $4, $5)
		ON CONFLICT (prefecture) DO UPDATE SET
		attempted_at = now(),
		outcome = EXCLUDED.outcome,
		consecutive_failures = EXCLUDED.consecutive_failures,
		failed_urls = EXCLUDED.failed_urls,
		requests_made = EXCLUDED.requests_made`,
		prefecture, attempt.Outcome, attempt.ConsecutiveFailures, attempt.FailedURLs, attempt.RequestsMade)
	if err != nil {
		log.Printf("%s: attempt not recorded: %v", prefecture, err)
	}
}

// nextAttempt は、今回の結果から次に残す試行の記録を作る。
func nextAttempt(prev *competitions.Attempt, windowStart time.Time, outcome string, failed []string, made int) competitions.Attempt {
	fails := 0
	if prev != nil && prev.AttemptedAt.Compare(windowStart) >= 0 {
		fails = prev.ConsecutiveFailures
	}
	switch outcome {
	case competitions.AttemptTransient, competitions.AttemptPermanent, competitions.AttemptRateLimited:
		fails++
	default:
		fails = 0
	}
	if failed == nil {
		failed = []string{}
	}
	return competitions.Attempt{
		Outcome:             outcome,
		ConsecutiveFailures: fails,
		FailedURLs:          failed,
		RequestsMade:        made,
	}
}
