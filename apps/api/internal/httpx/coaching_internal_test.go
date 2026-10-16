package httpx

import (
	"testing"
	"time"
)

func TestIsMinorTreatsTheBirthMonthAsLateAsPossible(t *testing.T) {
	birth := time.Date(2008, time.October, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		now  time.Time
		want bool
	}{
		// 2026年10月中は、誕生日が来たか分からないので18歳未満として扱う。
		{time.Date(2026, time.October, 31, 23, 59, 0, 0, tokyo), true},
		{time.Date(2026, time.November, 1, 0, 0, 0, 0, tokyo), false},
		// 日本時間で判定する（UTC ではまだ10月31日）。
		{time.Date(2026, time.October, 31, 15, 0, 0, 0, time.UTC), false},
		{time.Date(2025, time.December, 1, 0, 0, 0, 0, tokyo), true},
	}
	for _, c := range cases {
		if got := isMinor(birth, c.now); got != c.want {
			t.Errorf("isMinor(2008-10, %s) = %v, want %v", c.now, got, c.want)
		}
	}
	// 12月生まれは翌年1月1日に大人になる。
	december := time.Date(2008, time.December, 1, 0, 0, 0, 0, time.UTC)
	if !isMinor(december, time.Date(2026, time.December, 31, 12, 0, 0, 0, tokyo)) {
		t.Error("2008-12 should still be a minor on 2026-12-31")
	}
	if isMinor(december, time.Date(2027, time.January, 1, 0, 0, 0, 0, tokyo)) {
		t.Error("2008-12 should be an adult on 2027-01-01")
	}
}

func TestParseBirthMonth(t *testing.T) {
	now := time.Date(2026, time.October, 15, 12, 0, 0, 0, tokyo)
	if got, msg := parseBirthMonth("2009-04", now); msg != "" || !got.Equal(time.Date(2009, time.April, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("2009-04 = %v %q", got, msg)
	}
	if _, msg := parseBirthMonth("2026-10", now); msg != "" {
		t.Fatalf("this month: %q", msg)
	}
	for _, raw := range []string{"", "2026-11", "1925-09", "2009/04", "2009-13"} {
		if _, msg := parseBirthMonth(raw, now); msg == "" {
			t.Errorf("%q should be rejected", raw)
		}
	}
}

func TestGuardianLimitText(t *testing.T) {
	three := int32(3000)
	thousand := int32(1000)
	if got := guardianLimitText(&three); got != "月3,000円まで" {
		t.Fatalf("got %q", got)
	}
	if got := guardianLimitText(&thousand); got != "月1,000円まで" {
		t.Fatalf("got %q", got)
	}
	if got := guardianLimitText(nil); got != "上限なし" {
		t.Fatalf("got %q", got)
	}
	if got := formatYenNumber(500); got != "500" {
		t.Fatalf("got %q", got)
	}
}
