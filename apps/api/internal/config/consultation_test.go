package config

import "testing"

// 動画添削のあとの追加相談（600円）は販売を廃止した（本体 #569）。環境変数を立てても開かない。
func TestCoachingConsultationIsAlwaysClosed(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("SUPABASE_JWT_SECRET", "test")
	t.Setenv("STRIPE_SECRET_KEY", "")
	t.Setenv("COACHING_CONSULTATION_ENABLED", "1")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.CoachingConsultation {
		t.Fatal("consultation sales must stay closed")
	}
}
