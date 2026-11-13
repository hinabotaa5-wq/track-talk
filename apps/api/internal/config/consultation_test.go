package config

import "testing"

// 動画添削のあとの追加質問は COACHING_CONSULTATION_ENABLED で開閉する。開くのは運営者の指示があってから。
func TestCoachingConsultationFollowsEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("SUPABASE_JWT_SECRET", "test")
	t.Setenv("STRIPE_SECRET_KEY", "")
	t.Setenv("COACHING_CONSULTATION_ENABLED", "1")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !c.CoachingConsultation {
		t.Fatal("consultation sales must open when the flag is set")
	}
	t.Setenv("COACHING_CONSULTATION_ENABLED", "")
	c, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.CoachingConsultation {
		t.Fatal("consultation sales must stay closed without the flag")
	}
}
