package httpx

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// コーチプロフィール翻訳の API 呼び出し枠。sense_summary_budget.go と同方式（advisory lock + DB 予約）。
// 失敗しても枠は返さない。全体の上限に加えて、コーチごとの上限も同じトランザクションで数える。
// coach_id が NULL の既存行は全体の集計にだけ入る。
func (s *Server) reserveCoachTranslationAttempt(ctx context.Context, coachID uuid.UUID) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('coach-translation-budget', 0))`); err != nil {
		return false, err
	}
	var now time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM coach_profile_translation_attempts WHERE attempted_at < $1`, now.Add(-25*time.Hour)); err != nil {
		return false, err
	}
	var dayCount, minuteCount int64
	if err := tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE attempted_at > $1), count(*) FILTER (WHERE attempted_at > $2) FROM coach_profile_translation_attempts`, now.Add(-24*time.Hour), now.Add(-time.Minute)).Scan(&dayCount, &minuteCount); err != nil {
		return false, err
	}
	if dayCount >= int64(coachTranslationDailyLimit) || minuteCount >= int64(coachTranslationMinuteLimit) {
		return false, nil
	}
	var coachDayCount, coachMinuteCount int64
	if err := tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE attempted_at > $2), count(*) FILTER (WHERE attempted_at > $3) FROM coach_profile_translation_attempts WHERE coach_id = $1`, coachID, now.Add(-24*time.Hour), now.Add(-time.Minute)).Scan(&coachDayCount, &coachMinuteCount); err != nil {
		return false, err
	}
	if coachDayCount >= int64(coachTranslationCoachDailyLimit) || coachMinuteCount >= int64(coachTranslationCoachMinuteLimit) {
		return false, nil
	}
	if _, err := tx.Exec(ctx, `INSERT INTO coach_profile_translation_attempts (attempted_at, coach_id) VALUES ($1, $2)`, now, coachID); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
