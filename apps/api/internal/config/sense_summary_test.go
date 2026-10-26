package config

import "testing"

func TestSenseSummaryLimits(t *testing.T) {
	// Loadは他の機能の環境設定も読むため、この試験では支払い等を無効にする。
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("SUPABASE_JWT_SECRET", "test")
	t.Setenv("STRIPE_SECRET_KEY", "")
	for _, tc := range []struct {
		name, daily, minute   string
		wantDaily, wantMinute int
		wantErr               bool
	}{
		{"unset", "", "", 400, 10, false},
		{"explicit", "400", "8", 400, 8, false},
		{"zero explicitly disables", "0", "0", 0, 0, false},
		{"negative daily", "-1", "5", 0, 0, true},
		{"invalid minute", "200", "bad", 0, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SENSE_SUMMARY_DAILY_LIMIT", tc.daily)
			t.Setenv("SENSE_SUMMARY_MINUTE_LIMIT", tc.minute)
			c, err := Load()
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v", err)
			}
			if err == nil && (c.SenseSummaryDailyLimit != tc.wantDaily || c.SenseSummaryMinuteLimit != tc.wantMinute) {
				t.Fatalf("limits=%d/%d", c.SenseSummaryDailyLimit, c.SenseSummaryMinuteLimit)
			}
		})
	}
}
