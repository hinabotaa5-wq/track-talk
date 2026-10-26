package httpx

import (
	"context"
	"time"
)

// API呼び出しの直前に試行枠を予約し、失敗しても返さない。Geminiのクォータは失敗でも消費しうるため。
// 24時間の移動窓でGoogleの太平洋時間のリセットや夏時間をまたぐ集中利用も抑える。
func (s *Server) reserveSenseSummaryAttempt(ctx context.Context) (bool, error) {
	daily, minute := s.cfg.SenseSummaryDailyLimit, s.cfg.SenseSummaryMinuteLimit
	if daily == 0 && minute == 0 {
		return true, nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(context.Background())
	// 複数インスタンス・利用者間でも、数える処理と予約を直列化する。AIの通信中は保持しない。
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('sense-summary-budget', 0))`); err != nil {
		return false, err
	}
	var now, providerStart time.Time
	if err := tx.QueryRow(ctx, `WITH t AS (SELECT clock_timestamp() AS at)
SELECT at, date_trunc('day', at AT TIME ZONE 'America/Los_Angeles') AT TIME ZONE 'America/Los_Angeles' FROM t`).Scan(&now, &providerStart); err != nil {
		return false, err
	}
	retainedSince := now.Add(-24 * time.Hour)
	if providerStart.Before(retainedSince) {
		retainedSince = providerStart
	}
	if _, err := tx.Exec(ctx, `DELETE FROM sense_summary_attempts WHERE attempted_at < $1`, retainedSince); err != nil {
		return false, err
	}
	var dayCount, minuteCount, providerDayCount int64
	if err := tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE attempted_at > $1), count(*) FILTER (WHERE attempted_at > $2), count(*) FILTER (WHERE attempted_at >= $3) FROM sense_summary_attempts`, now.Add(-24*time.Hour), now.Add(-time.Minute), providerStart).Scan(&dayCount, &minuteCount, &providerDayCount); err != nil {
		return false, err
	}
	if (daily > 0 && (dayCount >= int64(daily) || providerDayCount >= int64(daily))) || (minute > 0 && minuteCount >= int64(minute)) {
		return false, nil
	}
	if _, err := tx.Exec(ctx, `INSERT INTO sense_summary_attempts (attempted_at) VALUES ($1)`, now); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
