package httpx

import (
	"strings"
	"testing"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
)

func TestIsMinorTreatsTheBirthMonthAsLateAsPossible(t *testing.T) {
	birth := time.Date(2008, time.October, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		now  time.Time
		want bool
	}{
		// 2026年10月中は、誕生日が来たか分からないので18歳未満として扱う。
		{time.Date(2026, time.October, 31, 23, 59, 0, 0, jst.Zone), true},
		{time.Date(2026, time.November, 1, 0, 0, 0, 0, jst.Zone), false},
		// 日本時間で判定する（UTC ではまだ10月31日）。
		{time.Date(2026, time.October, 31, 15, 0, 0, 0, time.UTC), false},
		{time.Date(2025, time.December, 1, 0, 0, 0, 0, jst.Zone), true},
	}
	for _, c := range cases {
		if got := isMinor(birth, c.now); got != c.want {
			t.Errorf("isMinor(2008-10, %s) = %v, want %v", c.now, got, c.want)
		}
	}
	// 12月生まれは翌年1月1日に大人になる。
	december := time.Date(2008, time.December, 1, 0, 0, 0, 0, time.UTC)
	if !isMinor(december, time.Date(2026, time.December, 31, 12, 0, 0, 0, jst.Zone)) {
		t.Error("2008-12 should still be a minor on 2026-12-31")
	}
	if isMinor(december, time.Date(2027, time.January, 1, 0, 0, 0, 0, jst.Zone)) {
		t.Error("2008-12 should be an adult on 2027-01-01")
	}
}

func TestParseBirthMonth(t *testing.T) {
	now := time.Date(2026, time.October, 15, 12, 0, 0, 0, jst.Zone)
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

func TestFormatYenNumber(t *testing.T) {
	if got := formatYenNumber(3000); got != "3,000" {
		t.Fatalf("got %q", got)
	}
	if got := formatYenNumber(500); got != "500" {
		t.Fatalf("got %q", got)
	}
}

// 同意のお願いと同意の受付のメールは、サポートプランの同意として書く。前の仕組みの月の上限・購入の通知と、
// サポートプランでは使わない PayPay には触れない。
func TestGuardianMailsAreForTheSupportPlan(t *testing.T) {
	at := time.Date(2026, time.November, 5, 12, 0, 0, 0, time.UTC)
	mails := map[string]string{
		"link": guardianLinkMail("parent@example.com", "たろう", "https://example.com/guardian/x").Text,
		"consented": guardianConsentedMail("parent@example.com", "たろう",
			db.GuardianConsent{ConsentedAt: &at, SelfPayLimitYen: 1500}, "https://example.com/guardian/y").Text,
	}
	for name, text := range mails {
		for _, want := range []string{"継続プラン", "クレジットカード", "たろう"} {
			if !strings.Contains(text, want) {
				t.Errorf("%s mail should contain %q:\n%s", name, want, text)
			}
		}
		for _, banned := range []string{"上限", "PayPay", "購入のたび", "購入するたび", "動画添削", "1,500"} {
			if strings.Contains(text, banned) {
				t.Errorf("%s mail should not contain %q:\n%s", name, banned, text)
			}
		}
	}
	if !strings.Contains(mails["consented"], "2026年11月5日 21:00") {
		t.Errorf("consented mail should show the consent time in JST:\n%s", mails["consented"])
	}
}
