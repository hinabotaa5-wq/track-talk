package httpx

import (
	"testing"
	"time"
)

func TestAvailabilityUsesTokyoDayAndInclusive21Days(t *testing.T) {
	before := availabilityToday(time.Date(2026, 10, 31, 14, 59, 59, 0, time.UTC))
	after := availabilityToday(time.Date(2026, 10, 31, 15, 0, 0, 0, time.UTC))
	if before.Format("2006-01-02") != "2026-10-31" || after.Format("2006-01-02") != "2026-11-01" {
		t.Fatal(before, after)
	}
	for _, tc := range []struct {
		date  string
		valid bool
	}{
		{"2026-10-31", false}, {"2026-11-01", true}, {"2026-11-21", true}, {"2026-11-22", false}, {"2026-11-31", false}, {"2026-11-1", false},
	} {
		if _, valid := availabilityDate(tc.date, after); valid != tc.valid {
			t.Errorf("%s: %v", tc.date, valid)
		}
	}
}
