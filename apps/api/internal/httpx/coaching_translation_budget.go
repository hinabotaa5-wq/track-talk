package httpx

import (
	"context"
	"time"
)

// 海外コーチとのやり取りの翻訳の API 呼び出し枠（本体 #916）。coach_translation_budget.go と同方式（advisory lock + DB 予約）。
// チャットの往復が多いため、上限はコーチプロフィール翻訳より広い（300回/日・30回/分）。失敗しても枠は返さない。
func (s *Server) reserveCoachingTranslationAttempt(ctx context.Context) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('coaching-translation-budget', 0))`); err != nil {
		return false, err
	}
	var now time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM coaching_translation_attempts WHERE attempted_at < $1`, now.Add(-25*time.Hour)); err != nil {
		return false, err
	}
	var dayCount, minuteCount int64
	if err := tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE attempted_at > $1), count(*) FILTER (WHERE attempted_at > $2) FROM coaching_translation_attempts`, now.Add(-24*time.Hour), now.Add(-time.Minute)).Scan(&dayCount, &minuteCount); err != nil {
		return false, err
	}
	if dayCount >= int64(coachingTranslationDailyLimit) || minuteCount >= int64(coachingTranslationMinuteLimit) {
		return false, nil
	}
	if _, err := tx.Exec(ctx, `INSERT INTO coaching_translation_attempts (attempted_at) VALUES ($1)`, now); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
