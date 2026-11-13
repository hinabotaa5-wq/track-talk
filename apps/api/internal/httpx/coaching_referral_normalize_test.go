package httpx

import "testing"

func TestNormalizeReferralCode(t *testing.T) {
	for _, input := range []string{"K7M3QX", "  k7m3qx ", "　ｋ７Ｍ３Ｑｘ　"} {
		if got := normalizeReferralCode(input); got != "K7M3QX" {
			t.Errorf("normalizeReferralCode(%q) = %q", input, got)
		}
	}
}
