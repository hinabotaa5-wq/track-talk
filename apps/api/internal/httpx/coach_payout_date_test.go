package httpx

import (
	"testing"
	"time"
)

func TestNextCoachPayoutDate(t *testing.T) {
	for _, tc := range []struct {
		utc     string
		payable bool
		want    string
	}{
		{"2026-11-14T15:00:00Z", true, "2026-11-15"},
		{"2026-11-15T15:00:00Z", true, "2026-12-15"},
		{"2026-11-15T15:00:00Z", false, "2026-12-15"},
		{"2026-12-15T15:00:00Z", true, "2027-01-15"},
	} {
		now, err := time.Parse(time.RFC3339, tc.utc)
		if err != nil {
			t.Fatal(err)
		}
		if got := nextCoachPayoutDate(now, tc.payable); got != tc.want {
			t.Errorf("%s payable=%v: got %s, want %s", tc.utc, tc.payable, got, tc.want)
		}
	}
}
